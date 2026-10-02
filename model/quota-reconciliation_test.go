package model_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"one-api/common/config"
	"one-api/model"
	"one-api/relay/relay_util"
	"one-api/types"
)

func reconciliationIntent() model.QuotaTerminal {
	known := 7
	return model.QuotaTerminal{Outcome: model.QuotaOutcomeReconcile, Reconciliation: &model.QuotaReconciliation{
		Version: 1, Reason: "realtime_missing_usage", ResponseHash: strings.Repeat("a", 64),
		Usage: &types.Usage{PromptTokens: 7}, KnownQuota: &known,
		PriceType: model.TokensPriceType, InputPrice: 1, OutputPrice: 1, GroupRatio: 1,
	}}
}

func TestQuotaTransactionReconciliationMigration(t *testing.T) {
	db, identity := quotaLedgerFixture(t, false, false)
	require.NoError(t, model.ReserveQuota("before-migration", identity, 20))
	// Model the previous ledger schema, with an existing live reservation.
	require.NoError(t, db.Migrator().DropColumn(&model.QuotaReservation{}, "ReconciliationData"))
	require.False(t, db.Migrator().HasColumn(&model.QuotaReservation{}, "ReconciliationData"))
	// A new binary starts with a new pool. Reusing the pre-downgrade pool here
	// would retain PostgreSQL SELECT * plans for the artificially removed column.
	previous := db
	reopened, err := gorm.Open(db.Dialector, &gorm.Config{NamingStrategy: db.Config.NamingStrategy})
	require.NoError(t, err)
	sqlDB, err := reopened.DB()
	require.NoError(t, err)
	db, model.DB = reopened, reopened
	t.Cleanup(func() { model.DB = previous; _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.QuotaReservation{}))
	require.True(t, db.Migrator().HasColumn(&model.QuotaReservation{}, "ReconciliationData"))
	var receipt model.QuotaReservation
	require.NoError(t, db.First(&receipt, "id = ?", "before-migration").Error)
	require.Equal(t, model.QuotaReservationReserved, receipt.State)
	require.Equal(t, 20, receipt.ReservedQuota)
	require.Empty(t, receipt.ReconciliationData)
	requireQuotaPair(t, db, identity.TokenID, 980, 980, 20)
	require.NoError(t, model.PrepareQuotaTerminal("before-migration", reconciliationIntent()))
	require.NoError(t, db.First(&receipt, "id = ?", "before-migration").Error)
	require.Equal(t, model.QuotaReservationReconcile, receipt.State)
	require.NotEmpty(t, receipt.ReconciliationData)
	requireQuotaPair(t, db, identity.TokenID, 980, 980, 20)
}

func TestQuotaTransactionReconciliationProtection(t *testing.T) {
	for _, unlimited := range []bool{false, true} {
		t.Run(fmt.Sprint(unlimited), func(t *testing.T) {
			db, identity := quotaLedgerFixture(t, false, unlimited)
			require.NoError(t, model.ReserveQuota("uncertain", identity, 20))
			intent := reconciliationIntent()
			require.NoError(t, model.PrepareQuotaTerminal("uncertain", intent))
			require.NoError(t, model.PrepareQuotaTerminal("uncertain", intent))
			require.ErrorIs(t, model.PrepareQuotaTerminal("uncertain", ledgerTerminal(7)), model.ErrQuotaReservationConflict)
			require.ErrorIs(t, model.PrepareQuotaTerminal("uncertain", model.QuotaTerminal{Outcome: model.QuotaOutcomeRefund}), model.ErrQuotaReservationConflict)
			require.ErrorIs(t, model.ReserveQuota("uncertain", identity, 40), model.ErrQuotaReservationConflict)
			require.ErrorIs(t, model.FinalizeQuotaReservation("uncertain"), model.ErrQuotaReservationConflict)
			*intent.Reconciliation.KnownQuota = 8
			require.ErrorIs(t, model.PrepareQuotaTerminal("uncertain", intent), model.ErrQuotaReservationConflict)
			reopened, err := gorm.Open(db.Dialector, &gorm.Config{NamingStrategy: db.Config.NamingStrategy})
			require.NoError(t, err)
			sqlDB, err := reopened.DB()
			require.NoError(t, err)
			model.DB = reopened
			t.Cleanup(func() { model.DB = db; _ = sqlDB.Close() })
			require.NoError(t, model.RecoverQuotaReservations(context.Background(), 100))
			var receipt model.QuotaReservation
			require.NoError(t, reopened.First(&receipt, "id = ?", "uncertain").Error)
			require.Equal(t, model.QuotaReservationReconcile, receipt.State)
			require.Empty(t, receipt.Outcome)
			require.Zero(t, receipt.FinalQuota)
			require.Empty(t, receipt.TerminalLog)
			var evidence model.QuotaReconciliation
			require.NoError(t, json.Unmarshal([]byte(receipt.ReconciliationData), &evidence))
			require.Equal(t, 7, *evidence.KnownQuota)
			require.Equal(t, 7, evidence.Usage.PromptTokens)
			tokenRemain, tokenUsed := 980, 20
			if unlimited {
				tokenRemain, tokenUsed = 1000, 0
			}
			requireQuotaPair(t, reopened, identity.TokenID, 980, tokenRemain, tokenUsed)
			var user model.User
			var logs int64
			require.NoError(t, reopened.First(&user, 1).Error)
			require.Zero(t, user.RequestCount)
			require.Zero(t, user.UsedQuota)
			require.NoError(t, reopened.Model(&model.Log{}).Count(&logs).Error)
			require.Zero(t, logs)
		})
	}
}

func TestQuotaTransactionReconciliationRetryAndConcurrentReplay(t *testing.T) {
	db, identity := quotaLedgerFixture(t, true, false)
	require.NoError(t, model.ReserveQuota("uncertain-retry", identity, 20))
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("reconcile-failure", func(tx *gorm.DB) {
		values, _ := tx.Statement.Dest.(map[string]any)
		if values["state"] == model.QuotaReservationReconcile {
			tx.AddError(errors.New("fixture evidence write unavailable"))
		}
	}))
	t.Cleanup(func() {
		_ = db.Callback().Update().Remove("reconcile-failure")
		_ = model.RecoverQuotaReservations(context.Background(), 100)
	})
	intent := reconciliationIntent()
	require.Error(t, model.SubmitQuotaTerminal("uncertain-retry", intent))
	intent.Reconciliation.Usage.PromptTokens = 999
	*intent.Reconciliation.KnownQuota = 999
	requireQuotaPair(t, db, identity.TokenID, 980, 980, 20)
	require.NoError(t, db.Callback().Update().Remove("reconcile-failure"))
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- model.RecoverQuotaReservations(context.Background(), 100) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var receipt model.QuotaReservation
	require.NoError(t, db.First(&receipt, "id = ?", "uncertain-retry").Error)
	require.Equal(t, model.QuotaReservationReconcile, receipt.State)
	var evidence model.QuotaReconciliation
	require.NoError(t, json.Unmarshal([]byte(receipt.ReconciliationData), &evidence))
	require.Equal(t, 7, evidence.Usage.PromptTokens)
	require.Equal(t, 7, *evidence.KnownQuota)
	requireQuotaPair(t, db, identity.TokenID, 980, 980, 20)
}

func TestQuotaRealtimeMissingUsageModes(t *testing.T) {
	for _, unlimited := range []bool{false, true} {
		for _, pricing := range []string{"tokens", "times", "free", "zero group"} {
			t.Run(fmt.Sprintf("%t/%s", unlimited, pricing), func(t *testing.T) {
				db, c := quotaLifecycleFixture(t, false, unlimited)
				price := model.PricingInstance.Prices["quota-fixture"]
				reserved, known := 20, 7
				switch pricing {
				case "times":
					price.Type, price.Input = model.TimesPriceType, .02
					known = 20
				case "free":
					price.Input, price.Output = 0, 0
					reserved, known = 0, 0
				case "zero group":
					c.Set("group_ratio", float64(0))
					reserved, known = 0, 0
				}
				q := relay_util.NewQuota(c, "quota-fixture", 0)
				require.Nil(t, q.PreRealtimeQuotaConsumption())
				total := &types.UsageEvent{}
				require.NoError(t, q.UpdateUserRealtimeQuota(total, &types.UsageEvent{ResponseID: "fixture-known", InputTokens: 7, InputTokenDetails: types.PromptTokensDetails{AudioTokens: 3}}))
				require.Error(t, q.UpdateUserRealtimeQuota(total, &types.UsageEvent{ResponseID: "raw-response-marker", MissingUsage: true}))
				require.True(t, q.NeedsRealtimeReconciliation())
				require.Error(t, q.UpdateUserRealtimeQuota(total, &types.UsageEvent{InputTokens: 9}))
				total.InputTokens = 999
				price.Input = 999
				q.Undo(c)
				q.Consume(c, total.ToChatUsage(), false)
				require.NoError(t, model.RecoverQuotaReservations(context.Background(), 100))
				quotaBalances(t, db, c, reserved, 0)
				var receipt model.QuotaReservation
				require.NoError(t, db.First(&receipt).Error)
				require.Equal(t, model.QuotaReservationReconcile, receipt.State)
				require.NotContains(t, receipt.ReconciliationData, "raw-response-marker")
				var evidence model.QuotaReconciliation
				require.NoError(t, json.Unmarshal([]byte(receipt.ReconciliationData), &evidence))
				require.Equal(t, 7, evidence.Usage.PromptTokens)
				require.Equal(t, 3, evidence.ExtraTokens[config.UsageExtraInputAudio])
				require.Equal(t, known, *evidence.KnownQuota)
			})
		}
	}
}
