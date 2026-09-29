package types_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"one-api/common/config"
	"one-api/types"
)

func TestUsageArithmeticRealtimeRepresentations(t *testing.T) {
	usage := &types.UsageEvent{}
	usage.Merge(&types.UsageEvent{InputTokens: 10, ExtraTokens: map[string]int{config.UsageExtraCache: 7}, InputTokenDetails: types.PromptTokensDetails{ImageTokens: 2}})
	usage.Merge(&types.UsageEvent{InputTokens: 10, InputTokenDetails: types.PromptTokensDetails{CachedTokens: 3, ImageTokens: 4}})
	result := usage.ToChatUsage()
	require.Equal(t, 20, result.PromptTokens)
	require.Equal(t, 10, result.GetExtraTokens()[config.UsageExtraCache])
	require.Equal(t, 6, result.PromptTokensDetails.ImageTokens)
	result.ExtraTokens[config.UsageExtraCache] = 123
	require.Equal(t, 10, usage.ToChatUsage().GetExtraTokens()[config.UsageExtraCache], "calculation snapshot must not mutate accumulated usage")
}

func TestUsageArithmeticRealtimeRejectsWithoutPartialMerge(t *testing.T) {
	for _, tc := range []struct {
		name          string
		current, next types.UsageEvent
	}{
		{"negative details", types.UsageEvent{InputTokens: 10}, types.UsageEvent{InputTokens: 1, InputTokenDetails: types.PromptTokensDetails{AudioTokens: -1}}},
		{"overflow details", types.UsageEvent{InputTokens: 10, InputTokenDetails: types.PromptTokensDetails{AudioTokens: math.MaxInt}}, types.UsageEvent{InputTokens: 1, InputTokenDetails: types.PromptTokensDetails{AudioTokens: 1}}},
		{"overflow extra map", types.UsageEvent{InputTokens: 10, ExtraTokens: map[string]int{config.UsageExtraCache: math.MaxInt}}, types.UsageEvent{InputTokens: 1, ExtraTokens: map[string]int{config.UsageExtraCache: 1}}},
		{"negative extra map", types.UsageEvent{InputTokens: 10}, types.UsageEvent{InputTokens: 1, ExtraTokens: map[string]int{config.UsageExtraCache: -1}}},
		{"sum overflow", types.UsageEvent{InputTokens: math.MaxInt}, types.UsageEvent{OutputTokens: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := tc.current.ToChatUsage()
			tc.current.Merge(&tc.next)
			require.Equal(t, before, tc.current.ToChatUsage())
		})
	}
}
