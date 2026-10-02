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

func TestRealtimeMissingUsageCannotFinalize(t *testing.T) {
	missing := `{"type":"response.done","response":{"id":"fixture-missing"}}`
	null := `{"type":"response.done","response":{"id":"fixture-null","usage":null}}`
	zero := `{"type":"response.done","response":{"id":"fixture-zero","usage":{}}}`
	known := `{"type":"response.done","response":{"id":"fixture-known","usage":{"input_tokens":7,"total_tokens":7}}}`
	for _, tc := range []struct {
		name, first  string
		later        []string
		uncertain    bool
		fee          int
		writeFailure bool
	}{
		{"missing initial", missing, nil, true, 0, false},
		{"null initial", null, nil, true, 0, false},
		{"missing after known", known, []string{missing}, true, 7, false},
		{"null after known", known, []string{null}, true, 7, false},
		{"response absent initial", `{"type":"response.done"}`, nil, true, 0, false},
		{"response null initial", `{"type":"response.done","response":null}`, nil, true, 0, false},
		{"response absent after known", known, []string{`{"type":"response.done"}`}, true, 7, false},
		{"response null after known", known, []string{`{"type":"response.done","response":null}`}, true, 7, false},
		{"same id later missing", known, []string{`{"type":"response.done","response":{"id":"fixture-known","usage":null}}`}, true, 7, false},
		{"anonymous missing", `{"type":"response.done","response":{}}`, nil, true, 0, false},
		{"missing first write failure", missing, nil, true, 0, true},
		{"explicit zero", zero, nil, false, 0, false},
		{"complete known", known, nil, false, 7, false},
		{"no-work handshake", `{"type":"session.created"}`, nil, false, 0, false},
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
				if len(tc.later) > 0 {
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
			_ = client.WriteMessage(websocket.TextMessage, []byte(missing))
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
				require.Equal(t, tc.fee, *evidence.KnownQuota)
				require.Equal(t, tc.fee, evidence.Usage.PromptTokens)
				require.NotContains(t, receipt.ReconciliationData, "fixture-known")
				require.NotContains(t, receipt.ReconciliationData, "fixture-missing")
				searchQuotaBalances(t, db, false, 20, 0)
			} else {
				require.Equal(t, model.QuotaReservationConsumed, receipt.State)
				searchQuotaBalances(t, db, false, tc.fee, 1)
			}
		})
	}
}

func TestRealtimeMissingUsageMetadataIsSupplierOwned(t *testing.T) {
	p := &openai.OpenAIProvider{}
	for _, source := range []requester.MessageSource{requester.UserMessage, requester.SupplierMessage} {
		for _, raw := range []string{
			`{"type":"response.done","response":{"usage":null}}`,
			`{"type":"response.done","response":{}}`,
			`{"type":"response.done","response":{"usage":{"MissingUsage":true,"missing_usage":true,"ResponseID":"spoof"}}}`,
		} {
			accepted, usage, _, err := p.HandleMessage(source, websocket.TextMessage, []byte(raw))
			require.NoError(t, err)
			require.True(t, accepted)
			if source == requester.UserMessage {
				require.Nil(t, usage)
				continue
			}
			require.NotNil(t, usage)
			require.Equal(t, !strings.Contains(raw, "spoof"), usage.MissingUsage)
			require.Empty(t, usage.ResponseID)
		}
	}
}
