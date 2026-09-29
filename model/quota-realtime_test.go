package model_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"one-api/common/config"
	"one-api/model"
	"one-api/relay/relay_util"
	"one-api/types"
)

func TestQuotaRealtimeFiniteBudget(t *testing.T) {
	for _, redisEnabled := range []bool{false, true} {
		for _, batch := range []bool{false, true} {
			t.Run(fmt.Sprintf("redis=%t/batch=%t", redisEnabled, batch), func(t *testing.T) {
				db, c := quotaLifecycleFixture(t, batch, false, redisEnabled)
				require.NoError(t, db.Model(&model.Token{}).Where("id=1").Update("remain_quota", 25).Error)
				q := relay_util.NewQuota(c, "quota-fixture", 0)
				require.Nil(t, q.PreQuotaConsumption())
				usage := &types.UsageEvent{}
				require.NoError(t, q.UpdateUserRealtimeQuota(usage, &types.UsageEvent{InputTokens: 7, TotalTokens: 7}))
				require.Error(t, q.UpdateUserRealtimeQuota(usage, &types.UsageEvent{InputTokens: 14, TotalTokens: 14}), "finite token budget must stop continuation")
				q.Consume(c, usage.ToChatUsage(), false)
				model.FlushQuotaBatchForTest()
				requireQuotaPair(t, db, 1, 979, 4, 21)
			})
		}
	}
}

func TestQuotaRealtimeSettlementControls(t *testing.T) {
	for _, redisEnabled := range []bool{false, true} {
		for _, unlimited := range []bool{false, true} {
			for _, pricing := range []string{"tokens", "fractional", "times", "free", "zero estimate"} {
				t.Run(fmt.Sprintf("redis=%t/unlimited=%t/%s", redisEnabled, unlimited, pricing), func(t *testing.T) {
					db, c := quotaLifecycleFixture(t, true, unlimited, redisEnabled)
					price := model.PricingInstance.Prices["quota-fixture"]
					spent := 28
					switch pricing {
					case "fractional":
						price.Input, price.Output = .1, .1
						spent = 3
					case "times":
						price.Type, price.Input = model.TimesPriceType, .02
						spent = 20
					case "free":
						price.Input, price.Output = 0, 0
						spent = 0
					case "zero estimate":
						config.PreConsumedQuota = 0
					}
					q := relay_util.NewQuota(c, "quota-fixture", 0)
					require.Nil(t, q.PreRealtimeQuotaConsumption())
					usage := &types.UsageEvent{}
					for _, n := range []int{7, 21} {
						require.NoError(t, q.UpdateUserRealtimeQuota(usage, &types.UsageEvent{InputTokens: n, TotalTokens: n}))
					}
					q.Consume(c, usage.ToChatUsage(), false)
					q.Undo(c)
					model.FlushQuotaBatchForTest()
					quotaBalances(t, db, c, spent, 1)
				})
			}
		}
	}
}

func TestQuotaRealtimeFailedTopUpKeepsReportedUsage(t *testing.T) {
	db, c := quotaLifecycleFixture(t, false, false)
	q := relay_util.NewQuota(c, "quota-fixture", 0)
	require.Nil(t, q.PreRealtimeQuotaConsumption())
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("realtime_write_error", func(tx *gorm.DB) { tx.AddError(errors.New("fixture write error")) }))
	usage := &types.UsageEvent{}
	require.Error(t, q.UpdateUserRealtimeQuota(usage, &types.UsageEvent{InputTokens: 25, TotalTokens: 25}))
	requireQuotaPair(t, db, 1, 980, 980, 20)
	require.NoError(t, db.Callback().Update().Remove("realtime_write_error"))
	q.Consume(c, usage.ToChatUsage(), false)
	quotaBalances(t, db, c, 25, 1)
}

func TestQuotaRealtimeFreeGroupNeedsNoReservation(t *testing.T) {
	db, c := quotaLifecycleFixture(t, false, false)
	c.Set("group_ratio", float64(0))
	require.NoError(t, db.Model(&model.User{}).Where("id=1").Update("quota", 19).Error)
	require.NoError(t, db.Model(&model.Token{}).Where("id=1").Update("remain_quota", 19).Error)
	q := relay_util.NewQuota(c, "quota-fixture", 0)
	require.Nil(t, q.PreRealtimeQuotaConsumption())
	require.False(t, q.HandelStatus)
	requireQuotaPair(t, db, 1, 19, 19, 0)
}
