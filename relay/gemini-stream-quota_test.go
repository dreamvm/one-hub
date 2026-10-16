package relay_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"one-api/common/requester"
	"one-api/model"
	"one-api/providers/gemini"
	"one-api/relay"
)

func TestGeminiStreamQuotaTerminal(t *testing.T) {
	const empty = "data: {\"candidates\":[{\"content\":{\"parts\":[]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":11,\"totalTokenCount\":11}}\n\n"
	const normal = "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"normal\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":20,\"candidatesTokenCount\":8,\"totalTokenCount\":28}}\n\n"
	for _, unlimited := range []bool{false, true} {
		for _, outcome := range []string{"normal", "retry-success", "builtin-empty", "two-empty", "retry-truncated"} {
			t.Run(outcome, func(t *testing.T) {
				t.Setenv("GEMINI_EMPTY_RESPONSE_RETRY_CHANNELS", "1")
				db, c, _, request := searchQuotaFixture(t, unlimited)
				recorder := httptest.NewRecorder()
				fresh, _ := gin.CreateTestContext(recorder)
				fresh.Keys, fresh.Request = c.Keys, c.Request
				c = fresh
				request.Stream = true
				var calls atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/event-stream")
					n := calls.Add(1)
					if outcome == "normal" || (n == 2 && outcome == "retry-success") {
						io.WriteString(w, normal)
					} else if n == 2 && outcome == "retry-truncated" {
						io.WriteString(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"partial\"}]}}],\"usageMetadata\":{\"promptTokenCount\":20,\"candidatesTokenCount\":8,\"totalTokenCount\":28}}\n\n")
					} else {
						io.WriteString(w, empty)
					}
				}))
				defer server.Close()
				oldClient := requester.HTTPClient
				requester.HTTPClient = server.Client()
				t.Cleanup(func() { requester.HTTPClient = oldClient })
				url, proxy := server.URL, ""
				channel := &model.Channel{Id: 1, BaseURL: &url, Proxy: &proxy, Key: "synthetic-fixture"}
				if outcome == "builtin-empty" {
					plugin := datatypes.NewJSONType(model.PluginType{"code_execution": {"enable": true}})
					channel.Plugin = &plugin
				}
				p := (&gemini.GeminiProviderFactory{}).Create(channel)
				p.SetContext(c)
				apiErr := relay.ExecuteChatForTest(c, p, request)
				wantCalls, spent, records := 2, 0, 0
				if outcome == "normal" || outcome == "retry-success" || outcome == "retry-truncated" {
					require.Nil(t, apiErr)
					spent, records = 39, 1
					if outcome == "normal" {
						spent, wantCalls = 28, 1
					}
				} else {
					require.NotNil(t, apiErr)
					if outcome == "builtin-empty" {
						wantCalls = 1
					}
				}
				if outcome == "retry-truncated" {
					// Preserve RC9's committed-stream settlement: known usage is
					// billed once, while the wire reports failure without DONE.
					require.Contains(t, recorder.Body.String(), "invalid_stream")
					require.NotContains(t, recorder.Body.String(), "[DONE]")
				}
				require.EqualValues(t, wantCalls, calls.Load())
				searchQuotaBalances(t, db, unlimited, spent, records)
				var reservations []model.QuotaReservation
				require.NoError(t, db.Find(&reservations).Error)
				require.Len(t, reservations, 1, "one client request owns one reservation, including a retry")
				state := model.QuotaReservationRefunded
				if records == 1 {
					state = model.QuotaReservationConsumed
				}
				require.Equal(t, state, reservations[0].State)
			})
		}
	}
}
