package relay_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"one-api/common"
	"one-api/common/requester"
	"one-api/model"
	"one-api/providers/gemini"
	"one-api/relay"
)

func TestGeminiMissingFinalUsageFallback(t *testing.T) {
	for _, tc := range []struct {
		name     string
		retry    bool
		thoughts int
		ending   string
	}{
		{"no-discard-control", false, 0, "normal"},
		{"discarded-prompt-only-control", true, 0, "normal"},
		{"discarded-thoughts-must-not-suppress-final-estimate", true, 7, "normal"},
		{"truncated-text-still-estimated", true, 7, "truncated"},
		{"invalid-json-after-text-still-estimated", true, 7, "invalid-json"},
		{"transport-interruption-after-text-still-estimated", true, 7, "transport"},
		{"final-snapshot-replaces-estimate", true, 7, "snapshot"},
		{"zero-completion-snapshot-preserves-fallback", true, 7, "zero-snapshot"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GEMINI_EMPTY_RESPONSE_RETRY_CHANNELS", "1")
			db, c, _, request := searchQuotaFixture(t, false)
			recorder := httptest.NewRecorder()
			fresh, _ := gin.CreateTestContext(recorder)
			fresh.Keys, fresh.Request = c.Keys, c.Request
			c = fresh
			request.Stream = true
			answer := strings.Repeat("synthetic answer ", 20)
			encoded, err := json.Marshal(answer)
			require.NoError(t, err)
			normal := fmt.Sprintf("data: {\"candidates\":[{\"index\":0,\"content\":{\"parts\":[{\"text\":%s}]},\"finishReason\":\"STOP\"}]}\n\n", encoded)
			if tc.ending == "truncated" || tc.ending == "transport" {
				normal = strings.Replace(normal, ",\"finishReason\":\"STOP\"", "", 1)
			}
			if tc.ending == "invalid-json" {
				normal += "data: {invalid\n\n"
			}
			if tc.ending == "snapshot" || tc.ending == "zero-snapshot" {
				completion := 8
				if tc.ending == "zero-snapshot" {
					completion = 0
				}
				normal += fmt.Sprintf("data: {\"usageMetadata\":{\"promptTokenCount\":20,\"candidatesTokenCount\":%d,\"totalTokenCount\":%d}}\n\n", completion, 20+completion)
			}
			empty := fmt.Sprintf("data: {\"candidates\":[{\"index\":0,\"content\":{\"parts\":[]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":11,\"thoughtsTokenCount\":%d,\"totalTokenCount\":%d}}\n\n", tc.thoughts, 11+tc.thoughts)
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				if calls.Add(1) == 1 && tc.retry {
					io.WriteString(w, empty)
				} else {
					io.WriteString(w, normal)
					if tc.ending == "transport" {
						w.(http.Flusher).Flush()
						conn, _, err := w.(http.Hijacker).Hijack()
						if err == nil {
							conn.Close()
						}
					}
				}
			}))
			defer server.Close()
			oldClient := requester.HTTPClient
			requester.HTTPClient = server.Client()
			t.Cleanup(func() { requester.HTTPClient = oldClient })
			url, proxy := server.URL, ""
			channel := &model.Channel{Id: 1, BaseURL: &url, Proxy: &proxy, Key: "synthetic-fixture"}
			provider := (&gemini.GeminiProviderFactory{}).Create(channel)
			provider.SetContext(c)
			input := common.CountTokenMessages(request.Messages, request.Model, channel.PreCost)
			output := common.CountTokenText(answer, request.Model)
			require.Positive(t, output)
			require.Nil(t, relay.ExecuteChatForTest(c, provider, request))
			if tc.ending == "truncated" || tc.ending == "transport" || tc.ending == "invalid-json" {
				require.NotContains(t, recorder.Body.String(), "[DONE]")
				require.Contains(t, recorder.Body.String(), "invalid_stream")
			} else {
				require.Contains(t, recorder.Body.String(), "[DONE]")
			}
			if tc.ending == "snapshot" || tc.ending == "zero-snapshot" {
				input = 20
			}
			if tc.ending == "snapshot" {
				output = 8
			}
			priorPrompt, wantCalls := 0, int32(1)
			if tc.retry {
				priorPrompt, wantCalls = 11, 2
			}
			require.Equal(t, wantCalls, calls.Load())
			usage := provider.GetUsage()
			t.Logf("final output estimate=%d, expected completion=%d, observed prompt/completion/total=%d/%d/%d", output, output+tc.thoughts, usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
			assert.Equal(t, output+tc.thoughts, usage.CompletionTokens, "discarded-attempt completion must not replace the final-attempt text fallback")
			assert.Equal(t, input+priorPrompt+output+tc.thoughts, usage.TotalTokens)
			searchQuotaBalances(t, db, false, input+priorPrompt+output+tc.thoughts, 1)
		})
	}
}
