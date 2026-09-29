package relay_util

import (
	"math"

	"one-api/common/config"
	"one-api/model"
	"one-api/types"
)

func finiteNonnegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

func (q *Quota) validPrice() bool {
	for _, value := range []float64{q.price.Input, q.price.Output, q.groupRatio, q.inputRatio, q.outputRatio} {
		if !finiteNonnegative(value) {
			return false
		}
	}
	if q.price.ExtraRatios != nil {
		for _, ratio := range q.price.ExtraRatios.Data() {
			if !finiteNonnegative(ratio) {
				return false
			}
		}
	}
	return !(q.groupRatio > 0 && ((q.price.GetInput() > 0 && q.inputRatio == 0) || (q.price.GetOutput() > 0 && q.outputRatio == 0)))
}

func quotaInteger(value float64) (int, bool) {
	if !finiteNonnegative(value) || value >= float64(math.MaxInt)+1 {
		return 0, false
	}
	return int(value), true
}

func addQuota(left, right int) (int, bool) {
	if right > 0 && left > math.MaxInt-right || right < 0 && left < math.MinInt-right {
		return 0, false
	}
	return left + right, true
}

// GetTotalQuota returns -1 for invalid arithmetic. Callers must never persist
// that sentinel as a fee or convert it to a refund.
func (q *Quota) GetTotalQuota(promptTokens, completionTokens int, extraBilling map[string]types.ExtraBilling) int {
	if !q.validPrice() || promptTokens < 0 || completionTokens < 0 || completionTokens > math.MaxInt-promptTokens {
		return -1
	}
	quota := 0
	hasTokens := promptTokens > 0 || completionTokens > 0
	if hasTokens {
		cost := math.Ceil(float64(promptTokens)*q.inputRatio + float64(completionTokens)*q.outputRatio)
		if q.price.Type == model.TimesPriceType {
			cost = 1000 * q.inputRatio
		}
		var ok bool
		quota, ok = quotaInteger(cost)
		if !ok {
			return -1
		}
	}
	q.GetExtraBillingData(extraBilling)
	extraQuota := 0
	for _, value := range q.extraBillingData {
		if value.CallCount < 0 || !finiteNonnegative(value.Price) || !finiteNonnegative(config.QuotaPerUnit) {
			return -1
		}
		if q.groupRatio == 0 || value.CallCount == 0 {
			continue
		}
		unit, ok := quotaInteger(math.Ceil(value.Price * config.QuotaPerUnit))
		if !ok {
			return -1
		}
		if unit == 0 && value.Price > 0 && config.QuotaPerUnit > 0 {
			unit = 1
		}
		if unit > 0 && value.CallCount > math.MaxInt/unit {
			return -1
		}
		extraQuota, ok = addQuota(extraQuota, unit*value.CallCount)
		if !ok {
			return -1
		}
	}
	if extraQuota > 0 {
		extra, ok := quotaInteger(math.Ceil(float64(extraQuota) * q.groupRatio))
		if !ok {
			return -1
		}
		quota, ok = addQuota(quota, extra)
		if !ok {
			return -1
		}
	}
	if hasTokens && quota == 0 && (q.inputRatio > 0 || (completionTokens > 0 && q.outputRatio > 0)) {
		quota = 1
	}
	return quota
}

func (q *Quota) getComputeTokensByUsage(usage *types.Usage) (int, int, bool) {
	if usage.Validate() != nil {
		return 0, 0, false
	}
	prompt, completion := usage.PromptTokens, usage.CompletionTokens
	extraTokens := usage.GetExtraTokens()
	if q.price.Type == model.TimesPriceType {
		hasUsage := prompt > 0 || completion > 0
		for _, value := range extraTokens {
			hasUsage = hasUsage || value > 0
		}
		if hasUsage {
			return 1, 0, true
		}
		return 0, 0, true
	}
	for key, value := range extraTokens {
		ratio := q.price.GetExtraRatio(key)
		if !finiteNonnegative(ratio) {
			return 0, 0, false
		}
		var adjustment int
		if ratio < 1 {
			// value + trunc(value*(ratio-1)) == ceil(value*ratio).
			// Compute the latter to avoid cancellation of tiny positive ratios.
			retained, ok := quotaInteger(math.Ceil(float64(value) * ratio))
			if !ok {
				return 0, 0, false
			}
			if retained == 0 && value > 0 && ratio > 0 {
				retained = 1
			}
			adjustment = retained - value
		} else {
			var ok bool
			adjustment, ok = quotaInteger(float64(value) * (ratio - 1))
			if !ok {
				return 0, 0, false
			}
		}
		var ok bool
		if model.GetExtraPriceIsPrompt(key) {
			prompt, ok = addQuota(prompt, adjustment)
		} else {
			completion, ok = addQuota(completion, adjustment)
		}
		if !ok {
			return 0, 0, false
		}
	}
	return prompt, completion, prompt >= 0 && completion >= 0
}

// GetTotalQuotaByUsage preserves -1 as an invalid-settlement sentinel.
func (q *Quota) GetTotalQuotaByUsage(usage *types.Usage) int {
	prompt, completion, ok := q.getComputeTokensByUsage(usage)
	if !ok {
		return -1
	}
	return q.GetTotalQuota(prompt, completion, usage.ExtraBilling)
}
