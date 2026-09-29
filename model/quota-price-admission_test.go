package model_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"one-api/common/config"
	"one-api/model"
	"one-api/relay/relay_util"
	"one-api/types"
)

func TestQuotaPriceAdmissionRejectsInvalidEstimate(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		input, output, group float64
		prompt, pre          int
		times                bool
	}{
		{"negative prompt", 1, 1, 1, -21, 20, false},
		{"negative precharge", 1, 1, 1, 0, -1, false},
		{"negative input", -1, 0, 1, 0, 0, false},
		{"negative output", 0, -1, 1, 0, 0, false},
		{"negative group", 1, 1, -1, 1, 0, false},
		{"nan input", math.NaN(), 0, 1, 0, 0, false},
		{"infinite output", 0, math.Inf(1), 1, 0, 0, false},
		{"infinite group", 1, 0, math.Inf(1), 0, 0, false},
		{"effective overflow", math.MaxFloat64, 0, 2, 0, 0, false},
		{"effective underflow", math.SmallestNonzeroFloat64, 0, .1, 0, 0, false},
		{"estimate overflow", math.MaxFloat64, 0, 1, 2, 0, false},
		{"addition overflow", 1, 0, 1, 1024, math.MaxInt, false},
		{"float integer boundary", 1, 0, 1, math.MaxInt, 0, false},
		{"times overflow", math.MaxFloat64, 0, 1, 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, c := quotaLifecycleFixture(t, false, false)
			p := model.PricingInstance.Prices["quota-fixture"]
			p.Input, p.Output = tc.input, tc.output
			if tc.times {
				p.Type = model.TimesPriceType
			}
			c.Set("group_ratio", tc.group)
			config.PreConsumedQuota = tc.pre
			q := relay_util.NewQuota(c, "quota-fixture", tc.prompt)
			require.NotNil(t, q.PreQuotaConsumption())
			require.False(t, q.HandelStatus)
			quotaBalances(t, db, c, 0, 0)
		})
	}
}

func TestQuotaPriceAdmissionPaidMinimum(t *testing.T) {
	for _, mode := range []string{"zero estimate", "fractional input", "output only", "tiny times"} {
		for _, empty := range []string{"user", "token", "neither"} {
			t.Run(mode+"/"+empty, func(t *testing.T) {
				db, c := quotaLifecycleFixture(t, false, false)
				config.PreConsumedQuota = 0
				p := model.PricingInstance.Prices["quota-fixture"]
				prompt := 0
				switch mode {
				case "fractional input":
					p.Input = .01
					prompt = 1
				case "output only":
					p.Input = 0
				case "tiny times":
					p.Type = model.TimesPriceType
					p.Input = .00001
				}
				user, token := 1000, 1000
				if empty == "user" {
					user = 0
				}
				if empty == "token" {
					token = 0
				}
				require.NoError(t, db.Model(&model.User{}).Where("id=1").Update("quota", user).Error)
				require.NoError(t, db.Model(&model.Token{}).Where("id=1").Update("remain_quota", token).Error)
				q := relay_util.NewQuota(c, "quota-fixture", prompt)
				err := q.PreQuotaConsumption()
				if empty != "neither" {
					require.NotNil(t, err)
					require.False(t, q.HandelStatus)
					requireQuotaPair(t, db, 1, user, token, 0)
				} else {
					require.Nil(t, err)
					require.True(t, q.HandelStatus)
					requireQuotaPair(t, db, 1, 999, 999, 1)
					q.Undo(c)
					quotaBalances(t, db, c, 0, 0)
				}
			})
		}
	}
}

func TestQuotaPriceAdmissionControls(t *testing.T) {
	for _, mode := range []string{"free model", "free group", "normal", "fractional", "times truncation", "zero usage refund"} {
		t.Run(mode, func(t *testing.T) {
			db, c := quotaLifecycleFixture(t, false, false)
			p := model.PricingInstance.Prices["quota-fixture"]
			pre, spent := 27, 7
			usage := &types.Usage{PromptTokens: 7}
			switch mode {
			case "free model":
				p.Input, p.Output = 0, 0
				pre, spent = 0, 0
			case "free group":
				c.Set("group_ratio", float64(0))
				pre, spent = 0, 0
			case "fractional":
				p.Input = .1
				pre, spent = 20, 1
			case "times truncation":
				p.Type = model.TimesPriceType
				p.Input = .0019
				pre, spent = 1, 1
			case "zero usage refund":
				usage = &types.Usage{}
				spent = 0
			}
			q := relay_util.NewQuota(c, "quota-fixture", 7)
			require.Nil(t, q.PreQuotaConsumption())
			requireQuotaPair(t, db, 1, 1000-pre, 1000-pre, pre)
			q.Consume(c, usage, false)
			quotaBalances(t, db, c, spent, 1)
		})
	}
}

func TestQuotaPriceAdmissionActualOverageBlocksNext(t *testing.T) {
	for _, redisEnabled := range []bool{false, true} {
		for _, batch := range []bool{false, true} {
			for _, limit := range []string{"user", "token"} {
				t.Run(fmt.Sprintf("redis=%t/batch=%t/%s", redisEnabled, batch, limit), func(t *testing.T) {
					db, c := quotaLifecycleFixture(t, batch, false, redisEnabled)
					config.PreConsumedQuota = 1
					user, token := 1000, 1000
					if limit == "user" {
						user = 5
					} else {
						token = 5
					}
					require.NoError(t, db.Model(&model.User{}).Where("id=1").Update("quota", user).Error)
					require.NoError(t, db.Model(&model.Token{}).Where("id=1").Update("remain_quota", token).Error)
					q := relay_util.NewQuota(c, "quota-fixture", 0)
					require.Nil(t, q.PreQuotaConsumption())
					q.Consume(c, &types.Usage{PromptTokens: 7}, false)
					model.FlushQuotaBatchForTest()
					requireQuotaPair(t, db, 1, user-7, token-7, 7)
					require.NotNil(t, relay_util.NewQuota(c, "quota-fixture", 0).PreQuotaConsumption())
					requireQuotaPair(t, db, 1, user-7, token-7, 7)
				})
			}
		}
	}
}
