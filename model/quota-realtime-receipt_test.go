package model_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"one-api/common/config"
	"one-api/model"
	"one-api/relay/relay_util"
	"one-api/types"
)

func TestQuotaRealtimeReceiptControls(t *testing.T) {
	for _, unlimited := range []bool{false, true} {
		for _, pricing := range []string{"tokens", "fractional", "times", "free"} {
			t.Run(fmt.Sprintf("unlimited=%t/%s", unlimited, pricing), func(t *testing.T) {
				db, c := quotaLifecycleFixture(t, true, unlimited)
				price := model.PricingInstance.Prices["quota-fixture"]
				spent := 14
				switch pricing {
				case "fractional":
					price.Input, price.Output = .1, .1
					spent = 2
				case "times":
					price.Type, price.Input = model.TimesPriceType, .02
					spent = 20
				case "free":
					price.Input, price.Output = 0, 0
					spent = 0
				}
				q := relay_util.NewQuota(c, "quota-fixture", 0)
				require.Nil(t, q.PreRealtimeQuotaConsumption())
				total := &types.UsageEvent{}
				for _, id := range []string{"response_a", "response_a", "response_b", "response_a", "response_b"} {
					require.NoError(t, q.UpdateUserRealtimeQuota(total, &types.UsageEvent{ResponseID: id, InputTokens: 7, TotalTokens: 7}))
				}
				require.Equal(t, 14, total.InputTokens)
				q.Consume(c, total.ToChatUsage(), false)
				quotaBalances(t, db, c, spent, 1)
			})
		}
	}
}

func TestQuotaRealtimeReceiptIsolationAndConflict(t *testing.T) {
	db, c := quotaLifecycleFixture(t, false, false)
	for i := 0; i < 2; i++ {
		q := relay_util.NewQuota(c, "quota-fixture", 0)
		require.Nil(t, q.PreRealtimeQuotaConsumption())
		total := &types.UsageEvent{}
		event := &types.UsageEvent{ResponseID: "same_id_across_sessions", InputTokens: 7, TotalTokens: 7}
		require.NoError(t, q.UpdateUserRealtimeQuota(total, event))
		// A different JSON-equivalent allocation must still be the same receipt.
		event.ExtraTokens = map[string]int{config.UsageExtraInputAudio: 0}
		require.NoError(t, q.UpdateUserRealtimeQuota(total, event))
		event.InputTokens = 8
		require.ErrorContains(t, q.UpdateUserRealtimeQuota(total, event), "conflicting realtime response usage")
		require.Equal(t, 7, total.InputTokens)
		q.Consume(c, total.ToChatUsage(), false)
	}
	quotaBalances(t, db, c, 14, 2)
}

func TestQuotaRealtimeReceiptExtraFieldsAndValidation(t *testing.T) {
	_, c := quotaLifecycleFixture(t, false, false)
	q := relay_util.NewQuota(c, "quota-fixture", 0)
	require.Nil(t, q.PreRealtimeQuotaConsumption())
	total := &types.UsageEvent{}
	event := &types.UsageEvent{ResponseID: "response_a", InputTokens: 7, TotalTokens: 7,
		InputTokenDetails: types.PromptTokensDetails{AudioTokens: 2}}
	require.NoError(t, q.UpdateUserRealtimeQuota(total, event))
	event.ExtraTokens = map[string]int{config.UsageExtraInputAudio: 2}
	require.NoError(t, q.UpdateUserRealtimeQuota(total, event))
	event.ExtraTokens[config.UsageExtraInputAudio] = 3
	require.ErrorContains(t, q.UpdateUserRealtimeQuota(total, event), "conflicting realtime response usage")
	require.Equal(t, 2, total.GetExtraTokens()[config.UsageExtraInputAudio])
	require.Error(t, q.UpdateUserRealtimeQuota(total, &types.UsageEvent{ResponseID: "invalid", InputTokens: -1}))
	require.NoError(t, q.UpdateUserRealtimeQuota(total, &types.UsageEvent{ResponseID: "invalid", InputTokens: 1, TotalTokens: 1}))
	require.Equal(t, 8, total.InputTokens, "invalid receipt must not poison a later valid report")
	q.Consume(c, total.ToChatUsage(), false)
}

func TestQuotaRealtimeReceiptLimitKeepsLastReportedUsage(t *testing.T) {
	db, c := quotaLifecycleFixture(t, false, false)
	q := relay_util.NewQuota(c, "quota-fixture", 0)
	require.Nil(t, q.PreRealtimeQuotaConsumption())
	total := &types.UsageEvent{}
	for i := 0; i < 16384; i++ {
		require.NoError(t, q.UpdateUserRealtimeQuota(total, &types.UsageEvent{ResponseID: fmt.Sprint(i)}))
	}
	// Existing zero receipt still deduplicates; entries must not be evicted.
	require.NoError(t, q.UpdateUserRealtimeQuota(total, &types.UsageEvent{ResponseID: "0"}))
	last := &types.UsageEvent{ResponseID: "limit", InputTokens: 7, TotalTokens: 7}
	require.ErrorContains(t, q.UpdateUserRealtimeQuota(total, last), "receipt limit")
	require.Equal(t, 7, total.InputTokens, "already reported usage cannot be erased by the bound")
	require.ErrorContains(t, q.UpdateUserRealtimeQuota(total, last), "receipt limit")
	require.Equal(t, 7, total.InputTokens)
	q.Consume(c, total.ToChatUsage(), false)
	quotaBalances(t, db, c, 7, 1)
}
