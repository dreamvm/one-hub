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
)

func TestRealtimeUnfinishedResponseCannotFinalize(t *testing.T) {
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
		{"started then lost", `{"type":"response.created","response":{"id":"fixture-active","status":"in_progress"}}`, nil, true, 0, false, false},
		{"started first write failure", `{"type":"response.created","response":{"id":"fixture-active","status":"in_progress"}}`, nil, true, 0, true, false},
		{"known then started lost", known, []string{`{"type":"response.created","response":{"id":"fixture-active","status":"in_progress"}}`}, true, 7, false, false},
		{"started then supplier error", `{"type":"response.created","response":{"id":"fixture-active","status":"in_progress"}}`, []string{`{"type":"error","error":{"type":"server_error","code":"fixture","message":"synthetic failure"}}`}, true, 0, false, false},
		{"two started one completed", `{"type":"response.created","response":{"id":"fixture-known","status":"in_progress"}}`, []string{`{"type":"response.created","response":{"id":"fixture-active","status":"in_progress"}}`, known}, true, 7, false, false},
		{"complete started response", `{"type":"response.created","response":{"id":"fixture-known","status":"in_progress"}}`, []string{known}, false, 7, false, false},
		{"cancelled with usage", `{"type":"response.created","response":{"id":"fixture-known","status":"in_progress"}}`, []string{`{"type":"response.done","response":{"id":"fixture-known","status":"cancelled","usage":{"input_tokens":7,"total_tokens":7}}}`}, false, 7, false, false},
		{"unsolicited complete", known, nil, false, 7, false, false},
		{"no-work handshake", `{"type":"session.created"}`, nil, false, 0, false, false},
		{"client closes active", `{"type":"response.created","response":{"id":"fixture-active"}}`, nil, true, 0, false, true},
		{"unrelated completion", `{"type":"response.created","response":{"id":"fixture-active"}}`, []string{known}, true, 7, false, false},
		{"anonymous start", `{"type":"response.created","response":{}}`, []string{known}, true, 7, false, false},
		{"anonymous completion cannot clear", `{"type":"response.created","response":{"id":"fixture-active"}}`, []string{`{"type":"response.done","response":{"usage":{"input_tokens":7}}}`}, true, 7, false, false},
		{"invalid completion cannot clear", `{"type":"response.created","response":{"id":"fixture-active"}}`, []string{`{"type":"response.done","response":{"id":"fixture-active","usage":{"input_tokens":-1}}}`}, true, 0, false, false},
		{"duplicate start control", `{"type":"response.created","response":{"id":"fixture-known"}}`, []string{`{"type":"response.created","response":{"id":"fixture-known"}}`, known}, false, 7, false, false},
		{"replayed start control", known, []string{`{"type":"response.created","response":{"id":"fixture-known"}}`, known}, false, 7, false, false},
		{"anonymous complete control", `{"type":"response.done","response":{"usage":{"input_tokens":7}}}`, nil, false, 7, false, false},
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

func TestRealtimeStartMetadataIsSupplierOwned(t *testing.T) {
	p := &openai.OpenAIProvider{}
	for _, raw := range []string{
		`{"type":"response.created","response":{"id":"fixture-active"}}`,
		`{"type":"response.created","response":null}`,
		`{"type":"response.created"}`,
	} {
		accepted, usage, _, err := p.HandleMessage(requester.UserMessage, websocket.TextMessage, []byte(raw))
		require.NoError(t, err)
		require.True(t, accepted)
		require.Nil(t, usage)
		accepted, usage, _, err = p.HandleMessage(requester.SupplierMessage, websocket.TextMessage, []byte(raw))
		require.NoError(t, err)
		require.True(t, accepted)
		require.True(t, usage.ResponseStarted)
	}
	_, usage, _, err := p.HandleMessage(requester.SupplierMessage, websocket.TextMessage, []byte(`{"type":"response.done","response":{"usage":{"ResponseStarted":true,"response_started":true}}}`))
	require.NoError(t, err)
	require.False(t, usage.ResponseStarted)
}
