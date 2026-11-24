package model

import (
	"context"
	"errors"
)

// ShutdownAccounting is called only after all HTTP work and background writers
// have joined. It replays existing determinate terminal intent, never invents a
// refund for a reserved/reconcile record, and fails on unknown or lost intent.
func ShutdownAccounting(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := RecoverQuotaReservations(ctx, 1000); err != nil {
			return errors.New("shutdown_quota_recovery_failed")
		}
		var pending int64
		if err := DB.WithContext(ctx).Model(&QuotaReservation{}).
			Where("state = ? OR compensation_state = ?", QuotaReservationPending, QuotaCompensationPending).
			Count(&pending).Error; err != nil {
			return errors.New("shutdown_quota_query_failed")
		}
		if pending == 0 {
			break
		}
	}
	retry := false
	quotaTerminalRetries.Range(func(_, _ any) bool { retry = true; return false })
	if retry {
		return errors.New("shutdown_terminal_retry_unresolved")
	}
	var unresolved int64
	if err := DB.WithContext(ctx).Model(&QuotaReservation{}).
		Where("state IS NULL OR compensation_state IS NULL OR state NOT IN ? OR compensation_state NOT IN ?", []string{QuotaReservationConsumed, QuotaReservationRefunded},
			[]string{"", QuotaCompensationApplied, QuotaCompensationNotCharged}).Count(&unresolved).Error; err != nil {
		return errors.New("shutdown_quota_query_failed")
	}
	if unresolved != 0 {
		return errors.New("shutdown_quota_unresolved")
	}
	return nil
}
