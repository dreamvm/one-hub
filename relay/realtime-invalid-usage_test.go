package relay_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"one-api/common/config"
	"one-api/common/requester"
	"one-api/model"
	"one-api/providers/openai"
	"one-api/relay"
	"one-api/types"
)

func TestRealtimeRejectedCompletionCannotFinalize(t *testing.T) {
	missing := `{"type":"response.done","response":{"id":"fixture-missing"}}`
	known := `{"type":"response.done","response":{"id":"fixture-known","usage":{"input_tokens":7,"total_tokens":7}}}`
	for _, tc := range []struct {
		name, first  string
		later        []string
		uncertain    bool
		fee          int
		writeFailure bool
		clientCloses bool
	}{
		{"malformed completion first", `{"type":"response.done","response":{"id":"fixture-active","usage":"invalid"}}`, nil, true, 0, false, false},
		{"malformed completion main", `{"type":"session.created"}`, []string{`{"type":"response.done","response":{"id":"fixture-active","usage":{"input_tokens":"7"}}}`}, true, 0, false, false},
		{"known then malformed", known, []string{`{"type":"response.done","response":{"id":"fixture-active","usage":{"input_tokens":1.5}}}`}, true, 7, false, false},
		{"negative completion main", `{"type":"session.created"}`, []string{`{"type":"response.done","response":{"id":"fixture-active","usage":{"input_token_details":{"audio_tokens":-1}}}}`}, true, 0, false, false},
		{"known then invalid counts", known, []string{`{"type":"response.done","response":{"id":"fixture-active","usage":{"input_tokens":-1}}}`}, true, 7, false, false},
		{"individual overflow completion", `{"type":"session.created"}`, []string{`{"type":"response.done","response":{"id":"fixture-active","usage":{"input_tokens":9223372036854775807,"output_tokens":1}}}`}, true, 0, false, false},

		{"malformed response object first", `{"type":"response.done","response":"invalid"}`, nil, true, 0, false, false},
		{"malformed response ID first", `{"type":"response.done","response":{"id":7,"usage":{"input_tokens":7}}}`, nil, true, 0, false, false},
		{"malformed start first", `{"type":"response.created","response":{"id":7}}`, nil, true, 0, false, false},
		{"malformed report anonymous", `{"type":"response.done","response":{"usage":[]}}`, nil, true, 0, false, false},
		{"malformed event metadata", `{"type":"response.done","event_id":7,"response":{"id":"fixture-active","usage":{}}}`, nil, true, 0, false, false},
		{"negative first", `{"type":"response.done","response":{"id":"fixture-active","usage":{"output_tokens":-1}}}`, nil, true, 0, false, false},
		{"malformed accepted ID replay", known, []string{`{"type":"response.done","response":{"id":"fixture-known","usage":"invalid"}}`}, false, 7, false, false},
		{"invalid accepted ID replay", known, []string{`{"type":"response.done","response":{"id":"fixture-known","usage":{"input_tokens":-1}}}`}, false, 7, false, false},
		{"control progress completion", `{"type":"response.text.delta","response_id":"fixture-known","delta":"hello"}`, []string{known}, false, 7, false, false},
		{"control content done completion", `{"type":"response.content_part.done","response_id":"fixture-known","part":{"type":"text","text":"hello"}}`, []string{known}, false, 7, false, false},
		{"control explicit zero", `{"type":"response.done","response":{"id":"fixture-known","usage":{}}}`, nil, false, 0, false, false},
		{"control no work handshake", `{"type":"session.created"}`, nil, false, 0, false, false},
		{"control receipt conflict", known, []string{`{"type":"response.done","response":{"id":"fixture-known","usage":{"input_tokens":8,"total_tokens":8}}}`}, false, 7, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, fixture, _, _ := searchQuotaFixture(t, false)
			model.PricingInstance.Prices["query-realtime"] = model.PricingInstance.Prices["query-fixture"]
			oldRetry := config.RetryTimes
			config.RetryTimes = 2
			t.Cleanup(func() { config.RetryTimes = oldRetry })
			var fail atomic.Bool
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()
				_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
				_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
				fail.Store(tc.writeFailure)
				if conn.WriteMessage(websocket.TextMessage, []byte(tc.first)) != nil {
					return
				}
				if len(tc.later) > 0 || tc.clientCloses {
					if _, _, err = conn.ReadMessage(); err != nil {
						return
					}
				}
				for _, frame := range tc.later {
					if conn.WriteMessage(websocket.TextMessage, []byte(frame)) != nil {
						return
					}
				}
			}))
			t.Cleanup(upstream.Close)
			require.NoError(t, db.Model(&model.Channel{}).Where("id=1").Updates(map[string]any{"type": config.ChannelTypeOpenAI, "status": config.ChannelStatusEnabled, "base_url": upstream.URL, "proxy": "", "key": "fixture-only"}).Error)
			fixture.Set("specific_channel_id", 1)
			done := make(chan struct{})
			router := gin.New()
			router.GET("/realtime", func(c *gin.Context) {
				defer close(done)
				for k, v := range fixture.Keys {
					c.Set(k, v)
				}
				relay.ChatRealtime(c)
			})
			gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				router.ServeHTTP(realtimeFailWriteResponse{ResponseWriter: w, fail: &fail}, r)
			}))
			t.Cleanup(gateway.Close)
			client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(gateway.URL, "http")+"/realtime?model=query-realtime", nil)
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = client.Close()
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Error("handler survived cleanup")
				}
			})
			_ = client.SetReadDeadline(time.Now().Add(3 * time.Second))
			_ = client.SetWriteDeadline(time.Now().Add(3 * time.Second))
			_, _, _ = client.ReadMessage()
			// Client-originated missing usage is never accounting metadata.
			if tc.clientCloses {
				_ = client.Close()
			} else {
				_ = client.WriteMessage(websocket.TextMessage, []byte(missing))
			}
			for {
				if _, _, err = client.ReadMessage(); err != nil {
					break
				}
			}
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("accounting did not finish")
			}
			var receipt model.QuotaReservation
			require.EqualValues(t, 1, calls.Load())
			require.NoError(t, db.First(&receipt).Error)
			if tc.uncertain {
				require.NotEqual(t, model.QuotaReservationConsumed, receipt.State, "missing usage must not create a determinate consumption")
				require.NotEqual(t, model.QuotaReservationRefunded, receipt.State, "missing usage must not refund")
				require.Empty(t, receipt.Outcome)
				require.Equal(t, model.QuotaReservationReconcile, receipt.State)
				var evidence model.QuotaReconciliation
				require.NoError(t, json.Unmarshal([]byte(receipt.ReconciliationData), &evidence))
				require.Equal(t, "realtime_unfinished_response", evidence.Reason)
				require.Equal(t, evidence.Reason, receipt.FailureCode)
				require.Equal(t, tc.fee, *evidence.KnownQuota)
				require.Equal(t, tc.fee, evidence.Usage.PromptTokens)
				require.NotContains(t, receipt.ReconciliationData, "fixture-known")
				require.NotContains(t, receipt.ReconciliationData, "fixture-missing")
				require.NotContains(t, receipt.ReconciliationData, "fixture-active")
				searchQuotaBalances(t, db, false, 20, 0)
			} else {
				require.Equal(t, model.QuotaReservationConsumed, receipt.State)
				searchQuotaBalances(t, db, false, tc.fee, 1)
			}
		})
	}
}

func TestRealtimeInvalidReportMetadata(t *testing.T) {
	p := &openai.OpenAIProvider{}
	for _, tc := range []struct{ name, raw, id string }{
		{"invalid usage", `{"type":"response.done","response":{"id":"fixture-a","usage":"invalid"}}`, "fixture-a"},
		{"escaped ID", `{"type":"response.done","response":{"id":"fixture-\u0061","usage":[]}}`, "fixture-a"},
		{"missing ID", `{"type":"response.done","response":{"usage":true}}`, ""},
		{"invalid ID", `{"type":"response.done","response":{"id":{},"usage":{}}}`, ""},
		{"no top level fallback", `{"type":"response.done","response_id":"fixture-a","response":7}`, ""},
		{"invalid start", `{"type":"response.created","response":{"id":"fixture-a","status":[]}}`, "fixture-a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, usage, replaced, err := p.HandleMessage(requester.SupplierMessage, websocket.TextMessage, []byte(tc.raw))
			require.Error(t, err)
			require.Nil(t, replaced)
			require.Equal(t, &types.UsageEvent{ResponseStarted: true, ResponseID: tc.id}, usage)
			event := err.(*types.Event)
			require.Equal(t, "json_unmarshal_failed", event.ErrorDetail.Type)
			require.Equal(t, "invalid_event", event.ErrorDetail.Code)
			_, usage, _, err = p.HandleMessage(requester.UserMessage, websocket.TextMessage, []byte(tc.raw))
			require.NoError(t, err)
			require.Nil(t, usage)
		})
	}
}
