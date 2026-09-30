package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"one-api/types"
)

func TestUsagePresenceAfterRealtimeMerge(t *testing.T) {
	for _, tc := range []struct {
		name     string
		reported *types.UsageEvent
		present  bool
	}{
		{"internal extra", &types.UsageEvent{ExtraTokens: map[string]int{"fixture-extra": 3}}, true},
		{"internal cache write", &types.UsageEvent{InputTokenDetails: types.PromptTokensDetails{CachedWriteTokens: 3}}, true},
		{"prediction detail", &types.UsageEvent{OutputTokenDetails: types.CompletionTokensDetails{AcceptedPredictionTokens: 3}}, true},
		{"explicit zero extra", &types.UsageEvent{ExtraTokens: map[string]int{"fixture-extra": 0}}, false},
		{"absent", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			total := &types.UsageEvent{}
			require.NoError(t, total.Merge(tc.reported))
			require.Equal(t, tc.present, total.ToChatUsage().HasTokenUsage())
		})
	}
}
