package model_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"one-api/model"
)

func quotaLedgerFixture(t *testing.T, batch, unlimited bool) (*gorm.DB, model.QuotaReservationIdentity) {
	t.Helper()
	db, token := quotaTransactionFixture(t, batch, unlimited)
	for _, table := range []any{&model.Channel{}, &model.Log{}, &model.QuotaReservation{}} {
		require.NoError(t, db.AutoMigrate(table))
	}
	t.Cleanup(func() {
		require.NoError(t, db.Migrator().DropTable(&model.QuotaReservation{}, &model.Log{}, &model.Channel{}))
	})
	require.NoError(t, db.Create(&model.Channel{Id: 1}).Error)
	return db, model.QuotaReservationIdentity{UserID: 1, TokenID: token.Id, UnlimitedQuota: unlimited, ChannelID: 1, ModelName: "ledger-fixture"}
}

func ledgerTerminal(amount int) model.QuotaTerminal {
	return model.QuotaTerminal{Outcome: model.QuotaOutcomeConsume, Quota: amount, RecordLog: true, Log: &model.Log{PromptTokens: 7, TokenName: "fixture-token"}}
}

func requireLedgerAccounting(t *testing.T, db *gorm.DB, identity model.QuotaReservationIdentity, spent, requests int) {
	t.Helper()
	tokenSpent := spent
	if identity.UnlimitedQuota {
		tokenSpent = 0
	}
	requireQuotaPair(t, db, identity.TokenID, 1000-spent, 1000-tokenSpent, tokenSpent)
	var user model.User
	var channel model.Channel
	var count int64
	require.NoError(t, db.First(&user, 1).Error)
	require.EqualValues(t, spent, user.UsedQuota)
	require.Equal(t, requests, user.RequestCount)
	require.NoError(t, db.First(&channel, 1).Error)
	require.EqualValues(t, spent, channel.UsedQuota)
	require.NoError(t, db.Model(&model.Log{}).Count(&count).Error)
	require.EqualValues(t, requests, count)
}

func TestQuotaTransactionLedgerReplayAndControls(t *testing.T) {
	for _, batch := range []bool{false, true} {
		for _, unlimited := range []bool{false, true} {
			for _, amount := range []int{0, 7, 20, 35} {
				t.Run(fmt.Sprintf("batch=%t/unlimited=%t/amount=%d", batch, unlimited, amount), func(t *testing.T) {
					db, identity := quotaLedgerFixture(t, batch, unlimited)
					require.NoError(t, model.ReserveQuota("same-reservation", identity, 20))
					require.NoError(t, model.ReserveQuota("same-reservation", identity, 20))
					terminal := ledgerTerminal(amount)
					require.NoError(t, model.PrepareQuotaTerminal("same-reservation", terminal))
					require.NoError(t, model.PrepareQuotaTerminal("same-reservation", terminal))
					require.NoError(t, model.FinalizeQuotaReservation("same-reservation"))
					// Simulate an ambiguous response after commit, then a fresh replay.
					terminal.Log.TokenName = "must-not-replace-first-snapshot"
					require.NoError(t, model.PrepareQuotaTerminal("same-reservation", terminal))
					require.NoError(t, model.FinalizeQuotaReservation("same-reservation"))
					require.ErrorIs(t, model.PrepareQuotaTerminal("same-reservation", model.QuotaTerminal{Outcome: model.QuotaOutcomeRefund}), model.ErrQuotaReservationConflict)
					require.Error(t, model.ReserveQuota("same-reservation", identity, 20))
					model.FlushQuotaBatchForTest()
					requireLedgerAccounting(t, db, identity, amount, 1)
					var log model.Log
					require.NoError(t, db.First(&log).Error)
					require.Equal(t, "fixture-token", log.TokenName)
					// Equal fees with different IDs are separate requests, never deduplicated.
					require.NoError(t, model.ReserveQuota("another-reservation", identity, 20))
					require.NoError(t, model.PrepareQuotaTerminal("another-reservation", ledgerTerminal(amount)))
					require.NoError(t, model.FinalizeQuotaReservation("another-reservation"))
					requireLedgerAccounting(t, db, identity, amount*2, 2)
				})
			}
		}
	}
}

func TestQuotaTransactionLedgerFreeAndRefund(t *testing.T) {
	for _, unlimited := range []bool{false, true} {
		for _, reserved := range []int{0, 20} {
			t.Run(fmt.Sprintf("unlimited=%t/reserved=%d", unlimited, reserved), func(t *testing.T) {
				db, identity := quotaLedgerFixture(t, true, unlimited)
				require.NoError(t, model.ReserveQuota("refund-fixture", identity, reserved))
				require.NoError(t, model.PrepareQuotaTerminal("refund-fixture", model.QuotaTerminal{Outcome: model.QuotaOutcomeRefund}))
				require.NoError(t, model.FinalizeQuotaReservation("refund-fixture"))
				require.NoError(t, model.FinalizeQuotaReservation("refund-fixture"))
				requireLedgerAccounting(t, db, identity, 0, 0)
				require.NoError(t, model.ReserveQuota("free-fixture", identity, 0))
				require.NoError(t, model.PrepareQuotaTerminal("free-fixture", ledgerTerminal(0)))
				require.NoError(t, model.FinalizeQuotaReservation("free-fixture"))
				requireLedgerAccounting(t, db, identity, 0, 1)
			})
		}
	}
}

func TestQuotaTransactionLedgerAtomicFailureRecovery(t *testing.T) {
	for _, stage := range []string{"user balance", "token", "channel", "log", "counter", "terminal state"} {
		t.Run(stage, func(t *testing.T) {
			db, identity := quotaLedgerFixture(t, true, false)
			require.NoError(t, model.ReserveQuota("failure-fixture", identity, 20))
			require.NoError(t, model.PrepareQuotaTerminal("failure-fixture", ledgerTerminal(7)))
			inject := func(tx *gorm.DB) {
				if tx.Statement.Schema == nil {
					return
				}
				table := tx.Statement.Schema.Table
				values, _ := tx.Statement.Dest.(map[string]interface{})
				fail := stage == "user balance" && strings.HasSuffix(table, "_users") && values["quota"] != nil ||
					stage == "token" && strings.HasSuffix(table, "_tokens") ||
					stage == "channel" && strings.HasSuffix(table, "_channels") ||
					stage == "log" && strings.HasSuffix(table, "_logs") ||
					stage == "counter" && strings.HasSuffix(table, "_users") && values["request_count"] != nil ||
					stage == "terminal state" && strings.HasSuffix(table, "_quota_reservations") && values["state"] == model.QuotaReservationConsumed
				if fail {
					tx.AddError(errors.New("injected ledger failure"))
				}
			}
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("ledger_fault", inject))
			require.NoError(t, db.Callback().Create().Before("gorm:create").Register("ledger_fault", inject))
			require.Error(t, model.FinalizeQuotaReservation("failure-fixture"))
			requireQuotaPair(t, db, identity.TokenID, 980, 980, 20)
			var user model.User
			var channel model.Channel
			var count int64
			var receipt model.QuotaReservation
			require.NoError(t, db.First(&user, 1).Error)
			require.Zero(t, user.RequestCount)
			require.Zero(t, user.UsedQuota)
			require.NoError(t, db.First(&channel, 1).Error)
			require.Zero(t, channel.UsedQuota)
			require.NoError(t, db.Model(&model.Log{}).Count(&count).Error)
			require.Zero(t, count)
			require.NoError(t, db.First(&receipt, "id = ?", "failure-fixture").Error)
			require.Equal(t, model.QuotaReservationPending, receipt.State)
			require.NoError(t, db.Callback().Update().Remove("ledger_fault"))
			require.NoError(t, db.Callback().Create().Remove("ledger_fault"))
			// Reopen a new database handle: recovery has no Quota object or usage input.
			reopened, err := gorm.Open(db.Dialector, &gorm.Config{NamingStrategy: db.NamingStrategy})
			require.NoError(t, err)
			reopenedSQL, err := reopened.DB()
			require.NoError(t, err)
			model.DB = reopened
			require.NoError(t, model.RecoverQuotaReservations(context.Background(), 100))
			require.NoError(t, model.RecoverQuotaReservations(context.Background(), 100))
			model.DB = db
			require.NoError(t, reopenedSQL.Close())
			requireLedgerAccounting(t, db, identity, 7, 1)
		})
	}
}

func TestQuotaTransactionLedgerConcurrentReplay(t *testing.T) {
	db, identity := quotaLedgerFixture(t, false, false)
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- model.ReserveQuota("concurrent", identity, 20) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	requireQuotaPair(t, db, identity.TokenID, 980, 980, 20)
	require.NoError(t, model.PrepareQuotaTerminal("concurrent", ledgerTerminal(7)))
	errs = make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- model.RecoverQuotaReservations(context.Background(), 100) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	requireLedgerAccounting(t, db, identity, 7, 1)
}

func TestQuotaTransactionLedgerTerminalRace(t *testing.T) {
	db, identity := quotaLedgerFixture(t, false, false)
	require.NoError(t, model.ReserveQuota("terminal-race", identity, 20))
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, terminal := range []model.QuotaTerminal{ledgerTerminal(7), {Outcome: model.QuotaOutcomeRefund}} {
		wg.Add(1)
		go func(terminal model.QuotaTerminal) {
			defer wg.Done()
			errs <- model.PrepareQuotaTerminal("terminal-race", terminal)
		}(terminal)
	}
	wg.Wait()
	close(errs)
	success := 0
	for err := range errs {
		if err == nil {
			success++
		} else {
			require.ErrorIs(t, err, model.ErrQuotaReservationConflict)
		}
	}
	require.Equal(t, 1, success)
	require.NoError(t, model.FinalizeQuotaReservation("terminal-race"))
	var receipt model.QuotaReservation
	require.NoError(t, db.First(&receipt, "id = ?", "terminal-race").Error)
	requests := 0
	if receipt.Outcome == model.QuotaOutcomeConsume {
		requests = 1
	}
	requireLedgerAccounting(t, db, identity, receipt.FinalQuota, requests)
}

func TestQuotaTransactionLedgerTopUpAndUnknown(t *testing.T) {
	db, identity := quotaLedgerFixture(t, false, false)
	require.NoError(t, model.ReserveQuota("top-up", identity, 20))
	require.NoError(t, model.ReserveQuota("top-up", identity, 40))
	require.NoError(t, model.ReserveQuota("top-up", identity, 40))
	require.NoError(t, model.ReserveQuota("top-up", identity, 20))
	requireQuotaPair(t, db, identity.TokenID, 960, 960, 40)
	require.Error(t, model.ReserveQuota("top-up", identity, 1001))
	requireQuotaPair(t, db, identity.TokenID, 960, 960, 40)
	changed := identity
	changed.TokenID++
	require.ErrorIs(t, model.ReserveQuota("top-up", changed, 40), model.ErrQuotaReservationConflict)
	require.NoError(t, db.Model(&model.QuotaReservation{}).Where("id = ?", "top-up").Update("created_at", 1).Error)
	require.NoError(t, model.RecoverQuotaReservations(context.Background(), 100))
	requireQuotaPair(t, db, identity.TokenID, 960, 960, 40)
	var receipt model.QuotaReservation
	require.NoError(t, db.First(&receipt, "id = ?", "top-up").Error)
	require.Equal(t, model.QuotaReservationReserved, receipt.State)
}

func TestQuotaTransactionLedgerIdentityAfterReservation(t *testing.T) {
	for _, change := range []string{"mode", "owner", "soft delete", "hard delete"} {
		t.Run(change, func(t *testing.T) {
			db, identity := quotaLedgerFixture(t, false, false)
			require.NoError(t, model.ReserveQuota("identity", identity, 20))
			switch change {
			case "mode":
				require.NoError(t, db.Model(&model.Token{}).Where("id = ?", identity.TokenID).Update("unlimited_quota", true).Error)
			case "owner":
				require.NoError(t, db.Model(&model.Token{}).Where("id = ?", identity.TokenID).Update("user_id", 2).Error)
			case "soft delete":
				require.NoError(t, db.Delete(&model.Token{}, identity.TokenID).Error)
			case "hard delete":
				require.NoError(t, db.Unscoped().Delete(&model.Token{}, identity.TokenID).Error)
			}
			require.NoError(t, model.PrepareQuotaTerminal("identity", ledgerTerminal(7)))
			err := model.FinalizeQuotaReservation("identity")
			if change == "mode" {
				require.NoError(t, err)
				requireLedgerAccounting(t, db, identity, 7, 1)
			} else {
				require.Error(t, err)
				var user model.User
				require.NoError(t, db.First(&user, 1).Error)
				require.Equal(t, 980, user.Quota)
				require.Zero(t, user.RequestCount)
				var receipt model.QuotaReservation
				require.NoError(t, db.First(&receipt, "id = ?", "identity").Error)
				require.Equal(t, model.QuotaReservationPending, receipt.State)
			}
		})
	}
}

func TestQuotaTransactionLedgerDisabledLogs(t *testing.T) {
	db, identity := quotaLedgerFixture(t, true, false)
	require.NoError(t, model.ReserveQuota("no-log", identity, 20))
	terminal := ledgerTerminal(7)
	terminal.RecordLog = false
	terminal.Log.SourceIp = "synthetic-private-value"
	require.NoError(t, model.PrepareQuotaTerminal("no-log", terminal))
	require.NoError(t, model.FinalizeQuotaReservation("no-log"))
	requireQuotaPair(t, db, identity.TokenID, 993, 993, 7)
	var count int64
	require.NoError(t, db.Model(&model.Log{}).Count(&count).Error)
	require.Zero(t, count)
	var receipt model.QuotaReservation
	require.NoError(t, db.First(&receipt, "id = ?", "no-log").Error)
	require.Empty(t, receipt.TerminalLog)
	var user model.User
	require.NoError(t, db.First(&user, 1).Error)
	require.Equal(t, 1, user.RequestCount)
}
