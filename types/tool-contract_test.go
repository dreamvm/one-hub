package types_test

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"one-api/types"
	"testing"
)

func TestParallelToolControlPresence(t *testing.T) {
	for _, input := range []string{`{}`, `{"parallel_tool_calls":false}`, `{"parallel_tool_calls":true}`} {
		var r types.ChatCompletionRequest
		require.NoError(t, json.Unmarshal([]byte(input), &r))
		b, e := json.Marshal(r)
		require.NoError(t, e)
		var output map[string]any
		require.NoError(t, json.Unmarshal(b, &output))
		if input == `{}` {
			require.Nil(t, r.ParallelToolCalls)
			require.NotContains(t, output, "parallel_tool_calls")
		} else {
			require.NotNil(t, r.ParallelToolCalls)
			require.Equal(t, *r.ParallelToolCalls, output["parallel_tool_calls"])
		}
		// Responses-to-chat conversion must preserve false versus unset too.
		var response types.OpenAIResponsesRequest
		require.NoError(t, json.Unmarshal([]byte(input), &response))
		require.Equal(t, r.ParallelToolCalls, response.ParallelToolCalls)
	}
}
