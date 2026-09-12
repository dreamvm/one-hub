package gemini_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"one-api/providers/gemini"
)

func TestGeminiStandardReasoningEffort(t *testing.T) {
	for _, effort := range []string{"minimal", "low", "medium", "high"} {
		t.Run(effort, func(t *testing.T) {
			r := geminiRequest(t, `"reasoning_effort":"`+effort+`","max_tokens":4096`)
			before, err := json.Marshal(r)
			require.NoError(t, err)
			g, e := gemini.ConvertFromChatOpenai(r)
			require.Nil(t, e)
			require.NotNil(t, g.GenerationConfig.ThinkingConfig)
			require.Equal(t, strings.ToUpper(effort), g.GenerationConfig.ThinkingConfig.ThinkingLevel)
			require.Nil(t, g.GenerationConfig.ThinkingConfig.ThinkingBudget, "effort must not silently inject a zero budget")
			after, err := json.Marshal(r)
			require.NoError(t, err)
			require.JSONEq(t, string(before), string(after))
		})
	}
}

func TestGeminiUnsupportedStandardReasoningEffort(t *testing.T) {
	for _, effort := range []string{"", "none", "xhigh", "LOW", "unknown"} {
		t.Run(effort, func(t *testing.T) {
			_, e := gemini.ConvertFromChatOpenai(geminiRequest(t, `"reasoning_effort":"`+effort+`"`))
			require.NotNil(t, e, "do not silently discard an explicit setting")
			require.Equal(t, http.StatusBadRequest, e.StatusCode)
			require.Contains(t, e.Message, "reasoning_effort")
			require.Equal(t, "invalid_reasoning_effort", e.Code)
		})
	}
}

func TestGeminiLegacyReasoningPrecedenceUnchanged(t *testing.T) {
	r := geminiRequest(t, `"reasoning_effort":"low","reasoning":{"effort":"high","max_tokens":-1}`)
	g, e := gemini.ConvertFromChatOpenai(r)
	require.Nil(t, e)
	require.NotNil(t, g.GenerationConfig.ThinkingConfig)
	require.Equal(t, "HIGH", g.GenerationConfig.ThinkingConfig.ThinkingLevel)
	require.Nil(t, g.GenerationConfig.ThinkingConfig.ThinkingBudget)
	g, e = gemini.ConvertFromChatOpenai(geminiRequest(t, `"max_tokens":4096`))
	require.Nil(t, e)
	require.Nil(t, g.GenerationConfig.ThinkingConfig, "omitted reasoning keeps upstream defaults")
}
