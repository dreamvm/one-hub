package model_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"one-api/model"
)

func TestShutdownAccountingCommittedAndReplay(t *testing.T) {
	db, id := quotaLedgerFixture(t, false, false)
	require.NoError(t, model.ShutdownAccounting(context.Background()))
	require.NoError(t, model.ReserveQuota("shutdown", id, 20))
	require.NoError(t, model.PrepareQuotaTerminal("shutdown", ledgerTerminal(7)))
	for i := 0; i < 2; i++ {
		require.NoError(t, model.ShutdownAccounting(context.Background()))
		requireLedgerAccounting(t, db, id, 7, 1)
	}
}

func TestShutdownAccountingNeverInfersRefund(t *testing.T) {
	for _, state := range []string{model.QuotaReservationReserved, model.QuotaReservationReconcile, "unknown"} {
		t.Run(state, func(t *testing.T) {
			db, id := quotaLedgerFixture(t, false, false)
			require.NoError(t, model.ReserveQuota("unresolved", id, 20))
			require.NoError(t, db.Model(&model.QuotaReservation{}).Where("id = ?", "unresolved").Update("state", state).Error)
			require.Error(t, model.ShutdownAccounting(context.Background()))
			var receipt model.QuotaReservation
			require.NoError(t, db.First(&receipt).Error)
			require.Equal(t, state, receipt.State)
			requireQuotaPair(t, db, id.TokenID, 980, 980, 20)
		})
	}
}

func TestShutdownAccountingReplaysMemoryTerminalIntent(t *testing.T) {
	db, id := quotaLedgerFixture(t, false, false)
	require.NoError(t, model.ReserveQuota("memory", id, 20))
	table := db.NamingStrategy.TableName("QuotaReservation")
	require.NoError(t, db.Exec(fmt.Sprintf("CREATE TRIGGER refuse_terminal BEFORE UPDATE ON `%s` WHEN NEW.state='pending' BEGIN SELECT RAISE(ABORT,'synthetic refusal'); END", table)).Error)
	require.Error(t, model.SubmitQuotaTerminal("memory", ledgerTerminal(7)))
	// The row alone remains reserved; the authoritative terminal intent is in
	// memory. Shutdown must replay it, never equate a quiet database to drained.
	require.Error(t, model.ShutdownAccounting(context.Background()))
	requireQuotaPair(t, db, id.TokenID, 980, 980, 20)
	require.NoError(t, db.Exec("DROP TRIGGER refuse_terminal").Error)
	require.NoError(t, model.ShutdownAccounting(context.Background()))
	requireLedgerAccounting(t, db, id, 7, 1)
	require.NoError(t, model.ShutdownAccounting(context.Background()))
	requireLedgerAccounting(t, db, id, 7, 1)
}

func TestShutdownAccountingContextAndUnknownCompensation(t *testing.T) {
	db, id := quotaLedgerFixture(t, false, false)
	require.NoError(t, model.ReserveQuota("deadline", id, 20))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.Error(t, model.ShutdownAccounting(ctx))
	require.NoError(t, model.PrepareQuotaTerminal("deadline", ledgerTerminal(7)))
	require.NoError(t, model.FinalizeQuotaReservation("deadline"))
	require.NoError(t, db.Model(&model.QuotaReservation{}).Where("id = ?", "deadline").Update("compensation_state", "unknown").Error)
	require.Error(t, model.ShutdownAccounting(context.Background()))
	requireLedgerAccounting(t, db, id, 7, 1)
}

func TestShutdownAccountingMissingAuthoritativeTable(t *testing.T) {
	db, _ := quotaLedgerFixture(t, false, false)
	table := db.NamingStrategy.TableName("QuotaReservation")
	require.NoError(t, db.Migrator().RenameTable(table, table+"_saved"))
	require.Error(t, model.ShutdownAccounting(context.Background()))
	require.NoError(t, db.Migrator().RenameTable(table+"_saved", table))
}

func TestShutdownAccountingNullStateCannotDisappearFromCount(t *testing.T) {
	for _, field := range []string{"state", "compensation_state"} {
		t.Run(field, func(t *testing.T) {
			db, id := quotaLedgerFixture(t, false, false)
			require.NoError(t, model.ReserveQuota("null", id, 20))
			require.NoError(t, model.PrepareQuotaTerminal("null", ledgerTerminal(7)))
			require.NoError(t, model.FinalizeQuotaReservation("null"))
			require.NoError(t, db.Model(&model.QuotaReservation{}).Where("id = ?", "null").Update(field, nil).Error)
			require.Error(t, model.ShutdownAccounting(context.Background()))
			var count int64
			require.NoError(t, db.Model(&model.QuotaReservation{}).Where(field+" IS NULL").Count(&count).Error)
			require.EqualValues(t, 1, count)
			requireQuotaPair(t, db, id.TokenID, 993, 993, 7)
		})
	}
}
