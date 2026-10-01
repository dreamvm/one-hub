package model_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"one-api/model"
	"one-api/relay/relay_util"
	"one-api/types"
)

func TestQuotaRealtimeUnfinishedModes(t *testing.T) {
	for _, unlimited := range []bool{false, true} {
		for _, pricing := range []string{"tokens", "times", "free", "zero group"} {
			t.Run(fmt.Sprintf("%t/%s", unlimited, pricing), func(t *testing.T) {
				db, c := quotaLifecycleFixture(t, false, unlimited)
				price := model.PricingInstance.Prices["quota-fixture"]
				reserved := 20
				switch pricing {
				case "times":
					price.Type, price.Input = model.TimesPriceType, .02
				case "free":
					price.Input, price.Output, reserved = 0, 0, 0
				case "zero group":
					c.Set("group_ratio", float64(0))
					reserved = 0
				}
				q := relay_util.NewQuota(c, "quota-fixture", 0)
				require.Nil(t, q.PreRealtimeQuotaConsumption())
				total := &types.UsageEvent{}
				require.NoError(t, q.UpdateUserRealtimeQuota(total, &types.UsageEvent{ResponseID: "fixture-active", ResponseStarted: true}))
				q.ReconcileUnfinishedRealtime(total)
				q.Undo(c)
				q.Consume(c, total.ToChatUsage(), false)
				require.NoError(t, model.RecoverQuotaReservations(context.Background(), 100))
				quotaBalances(t, db, c, reserved, 0)
				var receipt model.QuotaReservation
				require.NoError(t, db.First(&receipt).Error)
				require.Equal(t, model.QuotaReservationReconcile, receipt.State)
				require.Equal(t, "realtime_unfinished_response", receipt.FailureCode)
				var evidence model.QuotaReconciliation
				require.NoError(t, json.Unmarshal([]byte(receipt.ReconciliationData), &evidence))
				require.Equal(t, 1, evidence.UnfinishedResponses)
				require.Zero(t, *evidence.KnownQuota)
				require.NotContains(t, receipt.ReconciliationData, "fixture-active")
			})
		}
	}
}

func TestQuotaRealtimeCompleteBeforeTopUpFailure(t *testing.T) {
	db, c := quotaLifecycleFixture(t, false, false)
	q := relay_util.NewQuota(c, "quota-fixture", 0)
	require.Nil(t, q.PreRealtimeQuotaConsumption())
	total := &types.UsageEvent{}
	require.NoError(t, q.UpdateUserRealtimeQuota(total, &types.UsageEvent{ResponseID: "fixture-active", ResponseStarted: true}))
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("lifecycle-topup-failure", func(tx *gorm.DB) { tx.AddError(errors.New("fixture write error")) }))
	t.Cleanup(func() { _ = db.Callback().Update().Remove("lifecycle-topup-failure") })
	require.Error(t, q.UpdateUserRealtimeQuota(total, &types.UsageEvent{ResponseID: "fixture-active", InputTokens: 25}))
	require.NoError(t, db.Callback().Update().Remove("lifecycle-topup-failure"))
	q.ReconcileUnfinishedRealtime(total)
	require.False(t, q.NeedsRealtimeReconciliation())
	q.Consume(c, total.ToChatUsage(), false)
	quotaBalances(t, db, c, 25, 1)
}

func TestQuotaRealtimeActiveResponseCapacity(t *testing.T) {
	db, c := quotaLifecycleFixture(t, false, false)
	q := relay_util.NewQuota(c, "quota-fixture", 0)
	require.Nil(t, q.PreRealtimeQuotaConsumption())
	total := &types.UsageEvent{}
	for i := 0; i < 1024; i++ {
		require.NoError(t, q.UpdateUserRealtimeQuota(total, &types.UsageEvent{ResponseID: fmt.Sprint(i), ResponseStarted: true}))
	}
	require.Error(t, q.UpdateUserRealtimeQuota(total, &types.UsageEvent{ResponseID: "overflow", ResponseStarted: true}))
	q.ReconcileUnfinishedRealtime(total)
	q.Consume(c, total.ToChatUsage(), false)
	quotaBalances(t, db, c, 20, 0)
	var receipt model.QuotaReservation
	require.NoError(t, db.First(&receipt).Error)
	var evidence model.QuotaReconciliation
	require.NoError(t, json.Unmarshal([]byte(receipt.ReconciliationData), &evidence))
	require.Equal(t, 1024, evidence.UnfinishedResponses)
	require.True(t, evidence.UnattributedResponse)
	require.Less(t, len(receipt.ReconciliationData), 4096)
}

func TestQuotaTransactionUnfinishedResponsePersistence(t *testing.T) {
	db, identity := quotaLedgerFixture(t, false, false)
	require.NoError(t, model.ReserveQuota("unfinished-reopen", identity, 20))
	intent := reconciliationIntent()
	intent.Reconciliation.Reason = "realtime_unfinished_response"
	intent.Reconciliation.UnfinishedResponses = 2
	require.NoError(t, model.SubmitQuotaTerminal("unfinished-reopen", intent))
	reopened, err := gorm.Open(db.Dialector, &gorm.Config{NamingStrategy: db.Config.NamingStrategy})
	require.NoError(t, err)
	sqlDB, err := reopened.DB()
	require.NoError(t, err)
	model.DB = reopened
	t.Cleanup(func() { model.DB = db; _ = sqlDB.Close() })
	require.NoError(t, model.RecoverQuotaReservations(context.Background(), 100))
	var receipt model.QuotaReservation
	require.NoError(t, reopened.First(&receipt, "id = ?", "unfinished-reopen").Error)
	require.Equal(t, model.QuotaReservationReconcile, receipt.State)
	require.Equal(t, "realtime_unfinished_response", receipt.FailureCode)
	require.Empty(t, receipt.Outcome)
	requireQuotaPair(t, reopened, identity.TokenID, 980, 980, 20)
	var evidence model.QuotaReconciliation
	require.NoError(t, json.Unmarshal([]byte(receipt.ReconciliationData), &evidence))
	require.Equal(t, 2, evidence.UnfinishedResponses)
	require.Equal(t, 7, *evidence.KnownQuota)
}
