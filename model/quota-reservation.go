package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"one-api/common/config"
	"one-api/common/logger"
)

const (
	QuotaReservationReserved  = "reserved"
	QuotaReservationPending   = "pending"
	QuotaReservationConsumed  = "consumed"
	QuotaReservationRefunded  = "refunded"
	QuotaReservationReconcile = "reconcile"
	QuotaOutcomeConsume       = "consume"
	QuotaOutcomeRefund        = "refund"
	QuotaOutcomeReconcile     = "reconcile"
)

var ErrQuotaReservationConflict = errors.New("quota reservation operation conflicts with persisted state")

// This identity is captured before upstream work and cannot be changed by a
// terminal command. No credential or request body is stored in the ledger.
type QuotaReservationIdentity struct {
	UserID         int    `json:"user_id"`
	TokenID        int    `json:"token_id"`
	UnlimitedQuota bool   `json:"unlimited_quota"`
	ChannelID      int    `json:"channel_id"`
	ModelName      string `json:"model_name" gorm:"type:varchar(255)"`
	RequestID      string `json:"request_id" gorm:"type:varchar(64)"`
}

type QuotaReservation struct {
	CompensationState     string                   `json:"compensation_state" gorm:"type:varchar(16);index;default:''"`
	CompensationAttemptAt int64                    `json:"compensation_attempt_at" gorm:"index"`
	ID                    string                   `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Identity              QuotaReservationIdentity `json:"identity" gorm:"embedded"`
	ReservedQuota         int                      `json:"reserved_quota"`
	State                 string                   `json:"state" gorm:"type:varchar(16);index"`
	Outcome               string                   `json:"outcome" gorm:"type:varchar(16)"`
	FinalQuota            int                      `json:"final_quota"`
	TerminalLog           string                   `json:"-" gorm:"type:text"`
	ReconciliationData    string                   `json:"-" gorm:"type:text"`
	RecordLog             bool                     `json:"record_log"`
	CreatedAt             int64                    `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt             int64                    `json:"updated_at" gorm:"autoUpdateTime"`
	LastAttemptAt         int64                    `json:"last_attempt_at" gorm:"index"`
	FailureCode           string                   `json:"failure_code" gorm:"type:varchar(64)"`
	Revision              int64                    `json:"revision"`
}

type QuotaTerminal struct {
	Outcome        string
	Quota          int
	Log            *Log
	RecordLog      bool
	Reconciliation *QuotaReconciliation
}

// A real write acquires the row lock before reading it on all three engines.
// Unlike an UPDATE id=id, incrementing revision is not a MySQL no-op.
func lockQuotaReservation(tx *gorm.DB, id string) (*QuotaReservation, error) {
	result := tx.Model(&QuotaReservation{}).Where("id = ?", id).UpdateColumn("revision", gorm.Expr("revision + 1"))
	if err := quotaWriteResult(result, "quota reservation is missing"); err != nil {
		return nil, err
	}
	var receipt QuotaReservation
	if err := tx.First(&receipt, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &receipt, nil
}

func quotaReservationSubjects(tx *gorm.DB, identity QuotaReservationIdentity, checkMode bool) (User, error) {
	var user User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, identity.UserID).Error; err != nil {
		return user, err
	}
	query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", identity.TokenID, identity.UserID)
	if checkMode {
		query = query.Where("unlimited_quota = ?", identity.UnlimitedQuota)
	}
	var token Token
	return user, query.First(&token).Error
}

// ReserveQuota uses an absolute target so a retried initial reserve or Realtime
// top-up cannot debit twice. Decreasing targets never refund an active request.
func ReserveQuota(id string, identity QuotaReservationIdentity, target int) error {
	if id == "" || len(id) > 64 || identity.UserID <= 0 || identity.TokenID <= 0 || target < 0 {
		return errors.New("invalid quota reservation")
	}
	var user User
	delta := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		initial := QuotaReservation{ID: id, Identity: identity, State: QuotaReservationReserved}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&initial).Error; err != nil {
			return err
		}
		receipt, err := lockQuotaReservation(tx, id)
		if err != nil {
			return err
		}
		if receipt.Identity != identity || receipt.State != QuotaReservationReserved {
			return ErrQuotaReservationConflict
		}
		if target <= receipt.ReservedQuota {
			// Zero-cost admissions still bind the authenticated owner/mode.
			_, err = quotaReservationSubjects(tx, identity, true)
			return err
		}
		delta = target - receipt.ReservedQuota
		user, err = preConsumeTokenQuotaTx(tx, identity.TokenID, identity.UserID, identity.UnlimitedQuota, delta)
		if err != nil {
			return err
		}
		return tx.Model(receipt).Updates(map[string]any{"reserved_quota": target, "updated_at": time.Now().Unix()}).Error
	})
	if err != nil {
		return err
	}
	if delta > 0 {
		previous := user.Quota + delta
		tooLow := previous >= config.QuotaRemindThreshold && user.Quota < config.QuotaRemindThreshold
		if user.Email != "" && (tooLow || user.Quota <= 0) {
			go sendQuotaWarningEmail(user, previous, user.Quota <= 0)
		}
	}
	return nil
}

type quotaTerminalSnapshot struct {
	FailureCode string
	Outcome     string
	Quota       int
	Payload     string
	RecordLog   bool
}

// This retry set retains known outcomes while this process survives a database
// outage. It does not claim durability before Prepare succeeds; a crash in that
// window leaves the database reservation for explicit reconciliation.
var quotaTerminalRetries sync.Map

func snapshotQuotaTerminal(terminal QuotaTerminal) (quotaTerminalSnapshot, error) {
	if terminal.Outcome == QuotaOutcomeReconcile {
		return snapshotQuotaReconciliation(terminal)
	}
	if terminal.Quota < 0 || (terminal.Outcome != QuotaOutcomeConsume && terminal.Outcome != QuotaOutcomeRefund) || (terminal.Outcome == QuotaOutcomeRefund && terminal.Quota != 0) {
		return quotaTerminalSnapshot{}, errors.New("invalid quota terminal intent")
	}
	if terminal.Outcome == QuotaOutcomeConsume && terminal.Log == nil {
		return quotaTerminalSnapshot{}, errors.New("missing quota log snapshot")
	}
	payloadLog := terminal.Log
	if !terminal.RecordLog {
		payloadLog = nil
	}
	payload, err := json.Marshal(payloadLog)
	if err != nil {
		return quotaTerminalSnapshot{}, errors.New("invalid quota log snapshot")
	}
	return quotaTerminalSnapshot{Outcome: terminal.Outcome, Quota: terminal.Quota, Payload: string(payload), RecordLog: terminal.RecordLog}, nil
}

// PrepareQuotaTerminal returns success only after the chosen terminal intent is
// durable. Replays cannot replace the first amount, outcome or log snapshot.
func PrepareQuotaTerminal(id string, terminal QuotaTerminal) error {
	snapshot, err := snapshotQuotaTerminal(terminal)
	if err != nil {
		return err
	}
	return prepareQuotaTerminal(context.Background(), id, snapshot)
}

// SubmitQuotaTerminal additionally retains a retry if durable preparation fails.
// Stored snapshots are immutable and do not retain Gin contexts or usage objects.
func SubmitQuotaTerminal(id string, terminal QuotaTerminal) error {
	snapshot, err := snapshotQuotaTerminal(terminal)
	if err != nil {
		return err
	}
	first, _ := quotaTerminalRetries.LoadOrStore(id, snapshot)
	snapshot = first.(quotaTerminalSnapshot)
	err = prepareQuotaTerminal(context.Background(), id, snapshot)
	if err == nil || errors.Is(err, ErrQuotaReservationConflict) {
		quotaTerminalRetries.Delete(id)
	}
	return err
}

func prepareQuotaTerminal(ctx context.Context, id string, terminal quotaTerminalSnapshot) error {
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		receipt, err := lockQuotaReservation(tx, id)
		if err != nil {
			return err
		}
		if receipt.State != QuotaReservationReserved {
			if terminal.Outcome == QuotaOutcomeReconcile && receipt.State == QuotaReservationReconcile && receipt.ReconciliationData == terminal.Payload {
				return nil
			}
			if receipt.Outcome == terminal.Outcome && receipt.FinalQuota == terminal.Quota {
				return nil
			}
			return ErrQuotaReservationConflict
		}
		if terminal.Outcome == QuotaOutcomeReconcile {
			return tx.Model(receipt).Updates(map[string]any{"state": QuotaReservationReconcile, "reconciliation_data": terminal.Payload, "failure_code": terminal.FailureCode, "updated_at": time.Now().Unix()}).Error
		}
		return tx.Model(receipt).Updates(map[string]any{"state": QuotaReservationPending, "outcome": terminal.Outcome, "final_quota": terminal.Quota, "terminal_log": terminal.Payload, "record_log": terminal.RecordLog, "failure_code": "", "updated_at": time.Now().Unix()}).Error
	})
}

func FinalizeQuotaReservation(id string) error {
	return finalizeQuotaReservation(context.Background(), id)
}

// Balance, terminal state, counters and the enabled consume log commit together.
// The ledger path deliberately bypasses the legacy in-memory batch queues.
func finalizeQuotaReservation(ctx context.Context, id string) error {
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		receipt, err := lockQuotaReservation(tx, id)
		if err != nil {
			return err
		}
		if receipt.State == QuotaReservationConsumed || receipt.State == QuotaReservationRefunded {
			return nil
		}
		if receipt.State != QuotaReservationPending {
			return ErrQuotaReservationConflict
		}
		user, err := quotaReservationSubjects(tx, receipt.Identity, false)
		if err != nil {
			return err
		}
		if err := postConsumeTokenQuotaTx(tx, receipt.Identity.TokenID, receipt.Identity.UserID, receipt.Identity.UnlimitedQuota, receipt.FinalQuota-receipt.ReservedQuota); err != nil {
			return err
		}
		state := QuotaReservationRefunded
		if receipt.Outcome == QuotaOutcomeConsume {
			state = QuotaReservationConsumed
			var log Log
			if err := json.Unmarshal([]byte(receipt.TerminalLog), &log); err != nil {
				return errors.New("invalid persisted quota log")
			}
			log.Id, log.UserId, log.ChannelId, log.ModelName, log.Quota = 0, receipt.Identity.UserID, receipt.Identity.ChannelID, receipt.Identity.ModelName, receipt.FinalQuota
			log.Username, log.Type, log.Channel = user.Username, LogTypeConsume, nil
			result := tx.Model(&User{}).Where("id = ?", receipt.Identity.UserID).Updates(map[string]any{"used_quota": gorm.Expr("used_quota + ?", receipt.FinalQuota), "request_count": gorm.Expr("request_count + 1")})
			if err := quotaWriteResult(result, "quota user is missing"); err != nil {
				return err
			}
			if receipt.FinalQuota > 0 {
				if err := tx.Model(&Channel{}).Where("id = ?", receipt.Identity.ChannelID).Update("used_quota", gorm.Expr("used_quota + ?", receipt.FinalQuota)).Error; err != nil {
					return err
				}
			}
			if receipt.RecordLog {
				if err := tx.Omit("Channel").Create(&log).Error; err != nil {
					return err
				}
			}
		} else if receipt.Outcome != QuotaOutcomeRefund {
			return errors.New("invalid persisted quota outcome")
		}
		return tx.Model(receipt).Updates(map[string]any{"state": state, "terminal_log": "", "failure_code": "", "last_attempt_at": time.Now().Unix(), "updated_at": time.Now().Unix()}).Error
	})
	if err != nil {
		// Diagnostic only; a failed diagnostic write cannot change pending money.
		result := DB.WithContext(ctx).Model(&QuotaReservation{}).Where("id = ? AND state = ?", id, QuotaReservationPending).
			Updates(map[string]any{"failure_code": "terminal_write_failed", "last_attempt_at": time.Now().Unix()})
		if result.Error != nil {
			logger.SysError("failed to record quota recovery diagnostic")
		}
	}
	return err
}

// RecoverQuotaReservations only replays persisted, determinate terminal intent.
// Reserved records without that intent are left for reconciliation, never
// automatically refunded based on elapsed time or a dead process.
func RecoverQuotaReservations(ctx context.Context, limit int) error {
	if limit <= 0 || limit > 1000 {
		return errors.New("invalid quota recovery limit")
	}
	failed := 0
	attempted := 0
	quotaTerminalRetries.Range(func(key, value any) bool {
		if attempted >= limit || ctx.Err() != nil {
			return false
		}
		attempted++
		id := key.(string)
		err := prepareQuotaTerminal(ctx, id, value.(quotaTerminalSnapshot))
		if err == nil || errors.Is(err, ErrQuotaReservationConflict) {
			quotaTerminalRetries.Delete(id)
		}
		if err != nil {
			failed++
		}
		return true
	})
	var pending []QuotaReservation
	if err := DB.WithContext(ctx).Where("state = ?", QuotaReservationPending).Order("last_attempt_at ASC, created_at ASC, id ASC").Limit(limit).Find(&pending).Error; err != nil {
		return err
	}
	for _, receipt := range pending {
		if finalizeQuotaReservation(ctx, receipt.ID) != nil {
			failed++
		}
	}
	if RecoverQuotaCompensations(ctx, limit) != nil {
		failed++
	}
	if failed > 0 {
		return fmt.Errorf("%d quota reservations remain pending", failed)
	}
	return nil
}

func RunQuotaReservationRecovery(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		batchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := RecoverQuotaReservations(batchCtx, 100)
		cancel()
		if err != nil {
			logger.SysError("quota reservation recovery has pending errors")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
