package gemini_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiDiscardedTrailingUsageSnapshot(t *testing.T) {
	for _, trailing := range []bool{false, true} {
		name := "usage-on-finish-control"
		if trailing {
			name = "last-usage-only-event"
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("GEMINI_EMPTY_RESPONSE_RETRY_CHANNELS", "1")
			first := "data: {\"candidates\":[{\"index\":0,\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":11,\"thoughtsTokenCount\":7,\"totalTokenCount\":18}}\n\n"
			if trailing {
				first += "data: {\"usageMetadata\":{\"promptTokenCount\":12,\"thoughtsTokenCount\":9,\"totalTokenCount\":21}}\n\n"
			}
			second := "data: {\"candidates\":[{\"index\":0,\"content\":{\"parts\":[{\"text\":\"normal\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":20,\"candidatesTokenCount\":8,\"totalTokenCount\":28}}\n\n"
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				if calls.Add(1) == 1 {
					io.WriteString(w, first)
				} else {
					io.WriteString(w, second)
				}
			}))
			defer server.Close()
			provider := streamCompatProvider(t, server, context.Background(), false)
			stream, apiErr := provider.CreateChatCompletionStream(streamCompatRequest())
			require.Nil(t, apiErr)
			_, err := streamCompatDrain(stream)
			require.NoError(t, err)
			require.EqualValues(t, 2, calls.Load())
			prompt, completion, total := 31, 15, 46
			if trailing {
				prompt, completion, total = 32, 17, 49
			}
			t.Logf("observed prompt/completion/total=%d/%d/%d", provider.Usage.PromptTokens, provider.Usage.CompletionTokens, provider.Usage.TotalTokens)
			require.Equal(t, [3]int{prompt, completion, total}, [3]int{provider.Usage.PromptTokens, provider.Usage.CompletionTokens, provider.Usage.TotalTokens}, "last discarded-attempt cumulative usage must replace its earlier snapshot")
		})
	}
}
