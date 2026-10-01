package model_test

import (
	"github.com/stretchr/testify/require"
	"math"
	"one-api/model"
	"one-api/relay/relay_util"
	"one-api/types"
	"testing"
)

func TestQuotaRealtimeRejectedCompletionCannotFinalize(t *testing.T) {
	for _, tc := range []struct {
		name       string
		prior, bad *types.UsageEvent
	}{
		{"negative identified", nil, &types.UsageEvent{ResponseID: "bad", InputTokens: -1}},
		{"negative anonymous", nil, &types.UsageEvent{InputTokenDetails: types.PromptTokensDetails{AudioTokens: -1}}},
		{"internal extra", nil, &types.UsageEvent{ResponseID: "bad", ExtraTokens: map[string]int{"fixture": -1}}},
		{"cumulative overflow", &types.UsageEvent{ResponseID: "prior", TotalTokens: math.MaxInt}, &types.UsageEvent{ResponseID: "bad", TotalTokens: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, c := quotaLifecycleFixture(t, false, false)
			q := relay_util.NewQuota(c, "quota-fixture", 0)
			require.Nil(t, q.PreRealtimeQuotaConsumption())
			total := &types.UsageEvent{}
			if tc.prior != nil {
				require.NoError(t, q.UpdateUserRealtimeQuota(total, tc.prior))
			}
			require.Error(t, q.UpdateUserRealtimeQuota(total, tc.bad))
			q.ReconcileUnfinishedRealtime(total)
			q.Consume(c, total.ToChatUsage(), false)
			var r model.QuotaReservation
			require.NoError(t, db.First(&r).Error)
			t.Logf("actual state=%s outcome=%s final=%v total=%+v", r.State, r.Outcome, r.FinalQuota, total)
			require.Equal(t, model.QuotaReservationReconcile, r.State, "rejected completion cannot prove accepted total is final")
		})
	}
}

func TestQuotaRealtimeRejectedReportCorrection(t *testing.T) {
	for _, tc := range []struct {
		name, invalidID, validID string
		uncertain                bool
	}{
		{"matching correction", "fixture-a", "fixture-a", false},
		{"unrelated completion", "fixture-a", "fixture-b", true},
		{"anonymous rejection", "", "fixture-a", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, c := quotaLifecycleFixture(t, false, false)
			q := relay_util.NewQuota(c, "quota-fixture", 0)
			require.Nil(t, q.PreRealtimeQuotaConsumption())
			total := &types.UsageEvent{}
			require.Error(t, q.UpdateUserRealtimeQuota(total, &types.UsageEvent{ResponseID: tc.invalidID, InputTokens: -1}))
			require.NoError(t, q.UpdateUserRealtimeQuota(total, &types.UsageEvent{ResponseID: tc.validID, InputTokens: 7}))
			q.ReconcileUnfinishedRealtime(total)
			q.Consume(c, total.ToChatUsage(), false)
			var row model.QuotaReservation
			require.NoError(t, db.First(&row).Error)
			if tc.uncertain {
				require.Equal(t, model.QuotaReservationReconcile, row.State)
				quotaBalances(t, db, c, 20, 0)
			} else {
				require.Equal(t, model.QuotaReservationConsumed, row.State)
				quotaBalances(t, db, c, 7, 1)
			}
		})
	}
}
