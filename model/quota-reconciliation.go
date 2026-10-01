package model

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"

	"one-api/types"
)

// QuotaReconciliation is observed evidence, never a final bill or a command to
// change money. Raw events, response IDs, prompts and credentials are excluded.
type QuotaReconciliation struct {
	Version              int                `json:"version"`
	Reason               string             `json:"reason"`
	ResponseHash         string             `json:"response_hash,omitempty"`
	UnfinishedResponses  int                `json:"unfinished_responses,omitempty"`
	UnattributedResponse bool               `json:"unattributed_response,omitempty"`
	Usage                *types.Usage       `json:"usage"`
	ExtraTokens          map[string]int     `json:"extra_tokens"`
	KnownQuota           *int               `json:"known_quota"`
	PriceType            string             `json:"price_type"`
	InputPrice           float64            `json:"input_price"`
	OutputPrice          float64            `json:"output_price"`
	GroupRatio           float64            `json:"group_ratio"`
	ExtraRatios          map[string]float64 `json:"extra_ratios"`
}

func snapshotQuotaReconciliation(terminal QuotaTerminal) (quotaTerminalSnapshot, error) {
	r := terminal.Reconciliation
	if terminal.Quota != 0 || terminal.Log != nil || terminal.RecordLog || r == nil || r.Version != 1 {
		return quotaTerminalSnapshot{}, errors.New("invalid reconciliation intent")
	}
	if r.Reason != "realtime_missing_usage" && r.Reason != "realtime_unfinished_response" {
		return quotaTerminalSnapshot{}, errors.New("invalid reconciliation reason")
	}
	if r.UnfinishedResponses < 0 || r.UnfinishedResponses > 1024 || (r.Reason == "realtime_unfinished_response" && r.UnfinishedResponses == 0 && !r.UnattributedResponse) {
		return quotaTerminalSnapshot{}, errors.New("invalid unfinished response evidence")
	}
	if r.Usage.Validate() != nil || (r.KnownQuota != nil && *r.KnownQuota < 0) {
		return quotaTerminalSnapshot{}, errors.New("invalid reconciliation usage")
	}
	if r.ResponseHash != "" {
		decoded, err := hex.DecodeString(r.ResponseHash)
		if err != nil || len(decoded) != 32 {
			return quotaTerminalSnapshot{}, errors.New("invalid reconciliation response hash")
		}
	}
	for _, value := range r.ExtraTokens {
		if value < 0 {
			return quotaTerminalSnapshot{}, errors.New("invalid reconciliation extra usage")
		}
	}
	prices := []float64{r.InputPrice, r.OutputPrice, r.GroupRatio}
	for _, value := range r.ExtraRatios {
		prices = append(prices, value)
	}
	for _, value := range prices {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return quotaTerminalSnapshot{}, errors.New("invalid reconciliation price")
		}
	}
	payload, err := json.Marshal(r)
	if err != nil || len(payload) > 64*1024 {
		return quotaTerminalSnapshot{}, errors.New("invalid reconciliation snapshot")
	}
	return quotaTerminalSnapshot{Outcome: QuotaOutcomeReconcile, Payload: string(payload), FailureCode: r.Reason}, nil
}
