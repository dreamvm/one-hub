package relay_util

import (
	"crypto/sha256"
	"encoding/json"
	"errors"

	"one-api/types"
)

// Bound connection-local receipts without retaining arbitrarily large provider
// IDs or usage objects. A full connection stops; it must never evict old IDs
// and make their replay billable again.
const maxRealtimeReceipts = 16384

// The initial frame and supplier worker call this serially; final settlement
// waits for both WebSocket workers. These receipts are not crash recovery.
func (q *Quota) mergeRealtimeReceipt(total, incoming *types.UsageEvent) (bool, error) {
	if q.realtimeFull {
		return false, errors.New("realtime response receipt limit reached")
	}
	var id, digest [32]byte
	if incoming.ResponseID != "" {
		snapshot := incoming.ToChatUsage()
		if err := snapshot.Validate(); err != nil {
			return false, err
		}
		for key, value := range snapshot.GetExtraTokens() {
			if value == 0 {
				delete(snapshot.ExtraTokens, key)
			}
		}
		// Usage.ExtraTokens is excluded from protocol JSON, but still affects
		// billing. Include it explicitly in the receipt's immutable snapshot.
		encoded, err := json.Marshal(struct {
			Usage *types.Usage
			Extra map[string]int
		}{snapshot, snapshot.ExtraTokens})
		if err != nil {
			return false, err
		}
		id = sha256.Sum256([]byte(incoming.ResponseID))
		digest = sha256.Sum256(encoded)
		if previous, exists := q.realtimeReceipts[id]; exists {
			if previous != digest {
				return false, errors.New("conflicting realtime response usage")
			}
			return false, nil
		}
	}
	if err := total.Merge(incoming); err != nil {
		return false, err
	}
	if incoming.ResponseID != "" {
		if len(q.realtimeReceipts) == maxRealtimeReceipts {
			// This newly reported response already incurred usage. Keep it for
			// final settlement, then stop rather than forget any prior receipt.
			q.realtimeFull = true
			return true, errors.New("realtime response receipt limit reached")
		}
		if q.realtimeReceipts == nil {
			q.realtimeReceipts = make(map[[32]byte][32]byte)
		}
		q.realtimeReceipts[id] = digest
	}
	return true, nil
}
