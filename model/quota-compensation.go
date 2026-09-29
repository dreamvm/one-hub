package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"one-api/common/logger"
)

const (
	QuotaCompensationPending    = "pending"
	QuotaCompensationApplied    = "applied"
	QuotaCompensationNotCharged = "not_charged"
	TaskRefundRequested         = "requested"
	TaskRefundReviewRequired    = "review_required"
)

// Called in the same transaction as the first terminal task update. A task's
// stored estimate is never proof of a charge, and current token mode is not the
// mode under which the request paid. Missing historical evidence needs review.
func requestTaskCompensation(tx *gorm.DB, id string, userID, tokenID, channelID int) (string, error) {
	if id == "" {
		return TaskRefundReviewRequired, nil
	}
	receipt, err := lockQuotaReservation(tx, id)
	if err != nil {
		var count int64
		if queryErr := tx.Model(&QuotaReservation{}).Where("id = ?", id).Count(&count).Error; queryErr != nil {
			return "", queryErr
		}
		if count == 0 {
			return TaskRefundReviewRequired, nil
		}
		return "", err
	}
	if receipt.Identity.UserID != userID || receipt.Identity.TokenID != tokenID || receipt.Identity.ChannelID != channelID {
		return TaskRefundReviewRequired, nil
	}
	if receipt.CompensationState == "" {
		if err := tx.Model(receipt).Update("compensation_state", QuotaCompensationPending).Error; err != nil {
			return "", err
		}
	}
	return TaskRefundRequested, nil
}

// FinalizeQuotaCompensation is a separate, idempotent reversal of an original
// successful charge. It never changes the reservation's immutable outcome.
func FinalizeQuotaCompensation(id string) error {
	return finalizeQuotaCompensation(context.Background(), id)
}

func finalizeQuotaCompensation(ctx context.Context, id string) error {
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		receipt, err := lockQuotaReservation(tx, id)
		if err != nil {
			return err
		}
		if receipt.CompensationState != QuotaCompensationPending {
			return nil
		}
		if receipt.State == QuotaReservationRefunded {
			return tx.Model(receipt).Update("compensation_state", QuotaCompensationNotCharged).Error
		}
		if receipt.State != QuotaReservationConsumed {
			return errors.New("original quota charge remains unresolved")
		}
		user, err := quotaReservationSubjects(tx, receipt.Identity, false)
		if err != nil {
			return err
		}
		quota := receipt.FinalQuota
		maxInt := int(^uint(0) >> 1)
		if quota < 0 || user.Quota > maxInt-quota {
			return errors.New("quota compensation would overflow")
		}
		if !receipt.Identity.UnlimitedQuota {
			var token Token
			if err := tx.First(&token, receipt.Identity.TokenID).Error; err != nil {
				return err
			}
			if token.RemainQuota > maxInt-quota || token.UsedQuota < (-maxInt-1)+quota {
				return errors.New("token compensation would overflow")
			}
		}
		if err := postConsumeTokenQuotaTx(tx, receipt.Identity.TokenID, receipt.Identity.UserID, receipt.Identity.UnlimitedQuota, -quota); err != nil {
			return err
		}
		if quota > 0 {
			// Keep historical request/usage counters, with a separate compensating log.
			log := Log{UserId: receipt.Identity.UserID, Username: user.Username, ChannelId: receipt.Identity.ChannelID, CreatedAt: time.Now().Unix(), Type: LogTypeSystem, Content: fmt.Sprintf("异步任务执行失败，已补偿额度 %d", quota)}
			if err := tx.Create(&log).Error; err != nil {
				return err
			}
		}
		return tx.Model(receipt).Updates(map[string]any{"compensation_state": QuotaCompensationApplied, "updated_at": time.Now().Unix()}).Error
	})
	if err != nil {
		// Rotate unavailable subjects so one old record cannot starve newer refunds.
		if diagnosticErr := DB.WithContext(ctx).Model(&QuotaReservation{}).Where("id = ? AND compensation_state = ?", id, QuotaCompensationPending).Update("compensation_attempt_at", time.Now().Unix()).Error; diagnosticErr != nil {
			logger.SysError("failed to record quota compensation diagnostic")
		}
	}
	return err
}

func RecoverQuotaCompensations(ctx context.Context, limit int) error {
	if limit <= 0 || limit > 1000 {
		return errors.New("invalid quota compensation limit")
	}
	var receipts []QuotaReservation
	if err := DB.WithContext(ctx).Where("compensation_state = ?", QuotaCompensationPending).Order("compensation_attempt_at ASC, created_at ASC, id ASC").Limit(limit).Find(&receipts).Error; err != nil {
		return err
	}
	failed := 0
	for _, receipt := range receipts {
		if finalizeQuotaCompensation(ctx, receipt.ID) != nil {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d quota compensations remain pending", failed)
	}
	return nil
}
