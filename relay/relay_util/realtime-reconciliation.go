package relay_util

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"one-api/model"
	"one-api/types"
)

func (q *Quota) NeedsRealtimeReconciliation() bool {
	q.terminalMu.Lock()
	defer q.terminalMu.Unlock()
	return q.terminal != nil && q.terminal.Outcome == model.QuotaOutcomeReconcile
}

func (q *Quota) recordMissingRealtimeUsage(total *types.UsageEvent, responseID string) error {
	q.terminalMu.Lock()
	defer q.terminalMu.Unlock()
	if q.terminal != nil || q.terminalErr != nil {
		return errors.New("realtime accounting is already terminal")
	}
	usage := total.ToChatUsage()
	knownQuota := q.GetTotalQuotaByUsage(usage)
	evidence := &model.QuotaReconciliation{
		Version: 1, Reason: "realtime_missing_usage", Usage: usage,
		ExtraTokens: usage.GetExtraTokens(), PriceType: q.price.Type,
		InputPrice: q.price.GetInput(), OutputPrice: q.price.GetOutput(), GroupRatio: q.groupRatio,
		ExtraRatios: make(map[string]float64),
	}
	if knownQuota >= 0 {
		evidence.KnownQuota = &knownQuota
	}
	if responseID != "" {
		hash := sha256.Sum256([]byte(responseID))
		evidence.ResponseHash = hex.EncodeToString(hash[:])
	}
	for key := range model.ExtraKeyIsPrompt {
		evidence.ExtraRatios[key] = q.price.GetExtraRatio(key)
	}
	for key := range evidence.ExtraTokens {
		evidence.ExtraRatios[key] = q.price.GetExtraRatio(key)
	}
	q.terminal = &model.QuotaTerminal{Outcome: model.QuotaOutcomeReconcile, Reconciliation: evidence}
	return errors.New("realtime response usage missing; reservation requires reconciliation")
}
