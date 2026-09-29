package model_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"one-api/common/config"
	"one-api/model"
	"one-api/relay/relay_util"
	"one-api/types"
)

func TestQuotaSettlementArithmeticRejectsInvalidUsage(t *testing.T) {
	for _, tc := range []struct {
		name  string
		usage *types.Usage
		ratio float64
	}{
		{"negative prompt", &types.Usage{PromptTokens: -1}, 1},
		{"negative completion", &types.Usage{PromptTokens: 10, CompletionTokens: -1}, 1},
		{"negative total", &types.Usage{PromptTokens: 10, TotalTokens: -1}, 1},
		{"negative details", &types.Usage{PromptTokens: 10, PromptTokensDetails: types.PromptTokensDetails{AudioTokens: -1}}, 1},
		{"negative extra", &types.Usage{PromptTokens: 10, ExtraTokens: map[string]int{config.UsageExtraCache: -1}}, 1},
		{"token overflow", &types.Usage{PromptTokens: math.MaxInt, CompletionTokens: 1}, 1},
		{"adjustment overflow", &types.Usage{PromptTokens: 10, ExtraTokens: map[string]int{config.UsageExtraCache: math.MaxInt}}, 3},
		{"discount below zero", &types.Usage{PromptTokens: 1, ExtraTokens: map[string]int{config.UsageExtraCache: 100}}, 0},
		{"extra service negative", &types.Usage{PromptTokens: 10, ExtraBilling: map[string]types.ExtraBilling{types.APITollTypeFileSearch: {CallCount: -1}}}, 1},
		{"extra service overflow", &types.Usage{PromptTokens: 10, ExtraBilling: map[string]types.ExtraBilling{types.APITollTypeFileSearch: {CallCount: math.MaxInt}}}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, c := quotaLifecycleFixture(t, false, false)
			ratios := datatypes.NewJSONType(map[string]float64{config.UsageExtraCache: tc.ratio})
			model.PricingInstance.Prices["quota-fixture"].ExtraRatios = &ratios
			q := relay_util.NewQuota(c, "quota-fixture", 0)
			require.Nil(t, q.PreQuotaConsumption())
			require.Negative(t, q.GetTotalQuotaByUsage(tc.usage), "invalid arithmetic must not become a zero or small fee")
			q.Consume(c, tc.usage, false)
			requireQuotaPair(t, db, 1, 980, 980, 20)
			quotaBalances(t, db, c, 20, 0)
		})
	}
}

func TestQuotaSettlementArithmeticRejectsInvalidRatio(t *testing.T) {
	for _, ratio := range []float64{-1, math.NaN(), math.Inf(1)} {
		db, c := quotaLifecycleFixture(t, false, false)
		ratios := datatypes.NewJSONType(map[string]float64{config.UsageExtraCache: ratio})
		model.PricingInstance.Prices["quota-fixture"].ExtraRatios = &ratios
		q := relay_util.NewQuota(c, "quota-fixture", 0)
		require.NotNil(t, q.PreQuotaConsumption())
		quotaBalances(t, db, c, 0, 0)
	}
}

func TestQuotaSettlementArithmeticControls(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		input, output, group, ratio float64
		usage                       *types.Usage
		want                        int
		times                       bool
	}{
		{"fractional tokens", .1, .1, 1, 1, &types.Usage{PromptTokens: 7, CompletionTokens: 7}, 2, false},
		{"cached discount", 1, 1, 1, .1, &types.Usage{PromptTokens: 100, PromptTokensDetails: types.PromptTokensDetails{CachedTokens: 90}}, 19, false},
		{"free cached tokens", 1, 1, 1, 0, &types.Usage{PromptTokens: 100, PromptTokensDetails: types.PromptTokensDetails{CachedTokens: 100}}, 0, false},
		{"times truncation", .0019, 0, 1, 1, &types.Usage{PromptTokens: 1}, 1, true},
		{"tiny times", .00001, 0, 1, 1, &types.Usage{PromptTokens: 1}, 1, true},
		{"output only without output", 0, 1, 1, 1, &types.Usage{PromptTokens: 10}, 0, false},
		{"output only", 0, math.SmallestNonzeroFloat64, 1, 1, &types.Usage{CompletionTokens: 1}, 1, false},
		{"zero usage", 1, 1, 1, 1, &types.Usage{}, 0, false},
		{"free group", 1, 1, 0, 1, &types.Usage{PromptTokens: 10, ExtraBilling: map[string]types.ExtraBilling{types.APITollTypeFileSearch: {CallCount: 2}}}, 0, false},
		{"service only", 1, 1, 1, 1, &types.Usage{ExtraBilling: map[string]types.ExtraBilling{types.APITollTypeFileSearch: {CallCount: 2}}}, 2500, false},
		{"tokens and service", 1, 1, .5, 1, &types.Usage{PromptTokens: 10, ExtraBilling: map[string]types.ExtraBilling{types.APITollTypeFileSearch: {CallCount: 2}}}, 1255, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, c := quotaLifecycleFixture(t, false, false)
			p := model.PricingInstance.Prices["quota-fixture"]
			p.Input, p.Output = tc.input, tc.output
			if tc.times {
				p.Type = model.TimesPriceType
			}
			ratios := datatypes.NewJSONType(map[string]float64{config.UsageExtraCache: tc.ratio})
			p.ExtraRatios = &ratios
			c.Set("group_ratio", tc.group)
			q := relay_util.NewQuota(c, "quota-fixture", 0)
			require.Nil(t, q.PreQuotaConsumption())
			require.Equal(t, tc.want, q.GetTotalQuotaByUsage(tc.usage))
			q.Consume(c, tc.usage, false)
			quotaBalances(t, db, c, tc.want, 1)
		})
	}
}

func TestQuotaSettlementArithmeticDoesNotReuseExtraBilling(t *testing.T) {
	_, c := quotaLifecycleFixture(t, false, false)
	q := relay_util.NewQuota(c, "quota-fixture", 0)
	require.Equal(t, 1251, q.GetTotalQuotaByUsage(&types.Usage{PromptTokens: 1, ExtraBilling: map[string]types.ExtraBilling{types.APITollTypeFileSearch: {CallCount: 1}}}))
	require.Equal(t, 1, q.GetTotalQuotaByUsage(&types.Usage{PromptTokens: 1}))
}

func TestQuotaSettlementArithmeticExtraBillingVariants(t *testing.T) {
	for _, order := range []string{"file then image", "mixed images", "reverse mixed images"} {
		t.Run(order, func(t *testing.T) {
			_, c := quotaLifecycleFixture(t, false, false)
			usage := &types.Usage{PromptTokens: 1}
			want := 89001
			switch order {
			case "file then image":
				usage.IncExtraBilling(types.APITollTypeFileSearch, "")
				usage.IncExtraBilling(types.APITollTypeImageGeneration, "high-1024x1024")
				want = 84751
			case "mixed images":
				usage.IncExtraBilling(types.APITollTypeImageGeneration, "low-1024x1024")
				usage.IncExtraBilling(types.APITollTypeImageGeneration, "high-1024x1024")
			case "reverse mixed images":
				usage.IncExtraBilling(types.APITollTypeImageGeneration, "high-1024x1024")
				usage.IncExtraBilling(types.APITollTypeImageGeneration, "low-1024x1024")
			}
			require.Equal(t, want, relay_util.NewQuota(c, "quota-fixture", 0).GetTotalQuotaByUsage(usage))
		})
	}
}

func TestQuotaSettlementArithmeticTinyPositivePrices(t *testing.T) {
	t.Run("cached ratio", func(t *testing.T) {
		_, c := quotaLifecycleFixture(t, false, false)
		ratios := datatypes.NewJSONType(map[string]float64{config.UsageExtraCache: 1e-20})
		model.PricingInstance.Prices["quota-fixture"].ExtraRatios = &ratios
		q := relay_util.NewQuota(c, "quota-fixture", 0)
		require.Equal(t, 1, q.GetTotalQuotaByUsage(&types.Usage{PromptTokens: 10, PromptTokensDetails: types.PromptTokensDetails{CachedTokens: 10}}))
	})
	t.Run("service unit", func(t *testing.T) {
		_, c := quotaLifecycleFixture(t, false, false)
		old := config.QuotaPerUnit
		t.Cleanup(func() { config.QuotaPerUnit = old })
		config.QuotaPerUnit = math.SmallestNonzeroFloat64
		q := relay_util.NewQuota(c, "quota-fixture", 0)
		usage := &types.Usage{ExtraBilling: map[string]types.ExtraBilling{types.APITollTypeFileSearch: {CallCount: 1}}}
		require.Equal(t, 1, q.GetTotalQuotaByUsage(usage))
		config.QuotaPerUnit = 0
		require.Zero(t, q.GetTotalQuotaByUsage(usage))
	})
}

func TestQuotaSettlementArithmeticTimesIgnoresTokenDiscount(t *testing.T) {
	_, c := quotaLifecycleFixture(t, false, false)
	p := model.PricingInstance.Prices["quota-fixture"]
	p.Type = model.TimesPriceType
	p.Input = .02
	ratios := datatypes.NewJSONType(map[string]float64{config.UsageExtraCache: 0})
	p.ExtraRatios = &ratios
	q := relay_util.NewQuota(c, "quota-fixture", 0)
	require.Equal(t, 20, q.GetTotalQuotaByUsage(&types.Usage{PromptTokens: 100, PromptTokensDetails: types.PromptTokensDetails{CachedTokens: 100}}))
	require.Zero(t, q.GetTotalQuotaByUsage(&types.Usage{}))
}
