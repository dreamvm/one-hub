package relay_util

import (
	"crypto/sha256"
	"errors"

	"one-api/types"
)

const maxRealtimeActiveResponses = 1024

// Like receipts, lifecycle callbacks are serial: initial supplier frame, then
// supplier worker. Final inspection occurs only after that worker has stopped.
func (q *Quota) startRealtimeResponse(responseID string) error {
	if responseID == "" {
		// Anonymous parallel work cannot be matched to an arbitrary completion.
		q.realtimeUnattributed = true
		return nil
	}
	id := sha256.Sum256([]byte(responseID))
	if _, completed := q.realtimeReceipts[id]; completed {
		return nil
	}
	if _, active := q.realtimeActive[id]; active {
		return nil
	}
	if len(q.realtimeActive) == maxRealtimeActiveResponses {
		q.realtimeUnattributed = true
		return errors.New("realtime active response limit reached")
	}
	if q.realtimeActive == nil {
		q.realtimeActive = make(map[[32]byte]struct{})
	}
	q.realtimeActive[id] = struct{}{}
	return nil
}

func (q *Quota) completeRealtimeResponse(responseID string) {
	if responseID != "" {
		delete(q.realtimeActive, sha256.Sum256([]byte(responseID)))
	}
}

// ReconcileUnfinishedRealtime must run after supplier callbacks have stopped,
// including initial-frame failure, before either Consume or Undo is selected.
func (q *Quota) ReconcileUnfinishedRealtime(total *types.UsageEvent) {
	q.terminalMu.Lock()
	defer q.terminalMu.Unlock()
	if q.terminal != nil || q.terminalErr != nil || (len(q.realtimeActive) == 0 && !q.realtimeUnattributed) {
		return
	}
	q.selectRealtimeReconciliation(total, "", "realtime_unfinished_response")
}
