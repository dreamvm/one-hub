package relay_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"one-api/common/config"
	"one-api/common/logger"
	"one-api/common/requester"
	"one-api/model"
	"one-api/providers/openai"
	"one-api/relay"
	"one-api/types"
)

func TestRealtimeProgressWithoutStart(t *testing.T) {
	missing := `{"type":"response.done","response":{"id":"fixture-missing"}}`
	known := `{"type":"response.done","response":{"id":"fixture-known","usage":{"input_tokens":7,"total_tokens":7}}}`
	type progressCase struct {
		name, first  string
		later        []string
		uncertain    bool
		fee          int
		writeFailure bool
		clientCloses bool
	}
	var cases []progressCase
	for _, eventType := range []string{"response.text.delta", "response.text.done", "response.audio.delta", "response.audio.done", "response.audio_transcript.delta", "response.audio_transcript.done", "response.output_text.delta", "response.output_text.done", "response.output_audio.delta", "response.output_audio.done", "response.output_audio_transcript.delta", "response.output_audio_transcript.done", "response.content_part.added", "response.content_part.done", "response.output_item.added", "response.output_item.done", "response.function_call_arguments.delta", "response.function_call_arguments.done", "response.mcp_call_arguments.delta", "response.mcp_call_arguments.done"} {
		progress := fmt.Sprintf(`{"type":%q,"response_id":"fixture-active","delta":"fixture-output"}`, eventType)
		cases = append(cases,
			progressCase{eventType + "/first", progress, nil, true, 0, false, false},
			progressCase{eventType + "/after known", known, []string{progress}, true, 7, false, false},
			progressCase{eventType + "/complete control", progress, []string{`{"type":"response.done","response":{"id":"fixture-active","usage":{"input_tokens":7}}}`}, false, 7, false, false},
			progressCase{eventType + "/first write failure", progress, nil, true, 0, true, false},
			progressCase{eventType + "/client close", progress, nil, true, 0, false, true},
		)
	}
	progress := `{"type":"response.text.delta","response_id":"fixture-active","delta":"fixture-output"}`
	complete := `{"type":"response.done","response":{"id":"fixture-active","usage":{"input_tokens":7}}}`
	cases = append(cases,
		progressCase{"unrelated completion", progress, []string{known}, true, 7, false, false},
		progressCase{"repeated progress completes", progress, []string{progress, complete}, false, 7, false, false},
		progressCase{"completed progress replay", complete, []string{progress}, false, 7, false, false},
		progressCase{"malformed irrelevant field", `{"type":"response.text.delta","response_id":"fixture-active","response":{"usage":"invalid"}}`, nil, true, 0, false, false},
	)
	for _, rawID := range []string{`null`, `7`, `{}`, `[]`, `true`, `""`} {
		cases = append(cases, progressCase{"uncorrelatable " + rawID, fmt.Sprintf(`{"type":"response.text.delta","response_id":%s}`, rawID), []string{complete}, true, 7, false, false})
	}
	cases = append(cases, progressCase{"missing ID no nested fallback", `{"type":"response.text.delta","response":{"id":"fixture-active"}}`, []string{complete}, true, 7, false, false})
	for _, tc := range cases {
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
				require.NotEqual(t, model.QuotaReservationConsumed, receipt.State, "unresolved work must not create a determinate consumption")
				require.NotEqual(t, model.QuotaReservationRefunded, receipt.State, "unresolved work must not refund")
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

func TestRealtimeProgressMetadataBoundary(t *testing.T) {
	previousLogger := logger.Logger
	logger.Logger = zap.NewNop()
	t.Cleanup(func() { logger.Logger = previousLogger })
	p := &openai.OpenAIProvider{}
	for _, tc := range []struct{ name, fields, id string }{
		{"string", `"response_id":"fixture-active"`, "fixture-active"},
		{"escaped", `"response_id":"fixture-\u0061ctive"`, "fixture-active"},
		{"null", `"response_id":null`, ""},
		{"number", `"response_id":7`, ""},
		{"object", `"response_id":{}`, ""},
		{"array", `"response_id":[]`, ""},
		{"boolean", `"response_id":true`, ""},
		{"nested is not response ID", `"response":{"id":"fixture-active"}`, ""},
		{"item is not response ID", `"item_id":"fixture-active"`, ""},
		{"irrelevant malformed response", `"response_id":"fixture-active","response":{"usage":"invalid"}`, "fixture-active"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(`{"type":"response.output_text.delta",` + tc.fields + `}`)
			accepted, usage, replaced, err := p.HandleMessage(requester.SupplierMessage, websocket.TextMessage, raw)
			require.NoError(t, err)
			require.True(t, accepted)
			require.Nil(t, replaced)
			require.Equal(t, &types.UsageEvent{ResponseStarted: true, ResponseID: tc.id}, usage)
			for _, source := range []requester.MessageSource{requester.UserMessage, requester.SupplierMessage} {
				messageType := websocket.BinaryMessage
				if source == requester.UserMessage {
					messageType = websocket.TextMessage
				}
				accepted, usage, replaced, err = p.HandleMessage(source, messageType, raw)
				require.NoError(t, err)
				require.True(t, accepted)
				require.Nil(t, usage)
				require.Nil(t, replaced)
			}
		})
	}
	for _, eventType := range []string{"session.created", "session.updated", "conversation.item.created", "conversation.item.done", "input_audio_buffer.committed", "response.unknown", "response.mcp_call.in_progress"} {
		t.Run(eventType, func(t *testing.T) {
			raw := []byte(fmt.Sprintf(`{"type":%q,"response_id":"fixture-active","ResponseStarted":true}`, eventType))
			accepted, usage, replaced, err := p.HandleMessage(requester.SupplierMessage, websocket.TextMessage, raw)
			require.NoError(t, err)
			require.True(t, accepted)
			require.Nil(t, usage)
			require.Nil(t, replaced)
		})
	}
	t.Run("error contract", func(t *testing.T) {
		accepted, usage, replaced, err := p.HandleMessage(requester.SupplierMessage, websocket.TextMessage, []byte(`{"type":"error","error":{"message":"fixture error","code":"fixture"},"response_id":7}`))
		require.False(t, accepted)
		require.Nil(t, usage)
		require.Nil(t, replaced)
		require.IsType(t, &types.Event{}, err)
		require.NotContains(t, err.Error(), "response_id")
	})
}
