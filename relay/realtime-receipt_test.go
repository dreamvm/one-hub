package relay_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"one-api/common/config"
	"one-api/model"
	"one-api/relay"
)

func TestRealtimeResponseReceiptBilling(t *testing.T) {
	frame := func(eventID, responseID string, input int) string {
		return fmt.Sprintf(`{"type":"response.done","event_id":%q,"response":{"id":%q,"status":"completed","usage":{"input_tokens":%d,"total_tokens":%d}}}`, eventID, responseID, input, input)
	}
	for _, tc := range []struct {
		name     string
		first    string
		frames   []string
		spent    int
		conflict bool
	}{
		{"replayed event", "", []string{frame("event_a", "response_a", 7), frame("event_a", "response_a", 7)}, 7, false},
		{"response replay with new event id", "", []string{frame("event_a", "response_a", 7), frame("event_b", "response_a", 7)}, 7, false},
		{"initial response replay", frame("event_a", "response_a", 7), []string{frame("event_b", "response_a", 7)}, 7, false},
		{"equal usage distinct responses", "", []string{frame("event_a", "response_a", 7), frame("event_b", "response_b", 7)}, 14, false},
		{"anonymous legacy responses", "", []string{frame("event_a", "", 7), frame("event_b", "", 7)}, 14, false},
		{"conflicting response snapshot", "", []string{frame("event_a", "response_a", 7), frame("event_b", "response_a", 9)}, 7, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, fixture, _, _ := searchQuotaFixture(t, false)
			model.PricingInstance.Prices["query-realtime"] = model.PricingInstance.Prices["query-fixture"]
			oldRetry := config.RetryTimes
			config.RetryTimes = 1
			t.Cleanup(func() { config.RetryTimes = oldRetry })
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()
				_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
				_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
				first := tc.first
				if first == "" {
					first = `{"type":"session.created"}`
				}
				if conn.WriteMessage(websocket.TextMessage, []byte(first)) != nil {
					return
				}
				if _, _, err = conn.ReadMessage(); err != nil {
					return
				}
				for _, msg := range tc.frames {
					if conn.WriteMessage(websocket.TextMessage, []byte(msg)) != nil {
						return
					}
				}
				_, _, _ = conn.ReadMessage()
			}))
			defer upstream.Close()
			require.NoError(t, db.Model(&model.Channel{}).Where("id=1").Updates(map[string]any{
				"type": config.ChannelTypeOpenAI, "status": config.ChannelStatusEnabled,
				"base_url": upstream.URL, "proxy": "", "key": "fixture-only",
			}).Error)
			fixture.Set("specific_channel_id", 1)
			done := make(chan struct{})
			router := gin.New()
			router.GET("/realtime", func(c *gin.Context) {
				defer close(done)
				for key, value := range fixture.Keys {
					c.Set(key, value)
				}
				relay.ChatRealtime(c)
			})
			gateway := httptest.NewServer(router)
			defer gateway.Close()
			client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(gateway.URL, "http")+"/realtime?model=query-realtime", nil)
			require.NoError(t, err)
			defer client.Close()
			t.Cleanup(func() {
				_ = client.Close()
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Error("handler survived fixture cleanup")
				}
			})
			_ = client.SetReadDeadline(time.Now().Add(3 * time.Second))
			_ = client.SetWriteDeadline(time.Now().Add(3 * time.Second))
			_, _, err = client.ReadMessage()
			require.NoError(t, err)
			// A client-originated usage lookalike must not enter accounting.
			require.NoError(t, client.WriteMessage(websocket.TextMessage, []byte(frame("client_event", "client_response", 99))))
			for _, expected := range tc.frames {
				_, msg, err := client.ReadMessage()
				require.NoError(t, err)
				require.JSONEq(t, expected, string(msg), "provider frames remain visible, including repeated receipts")
			}
			if tc.conflict {
				_, msg, err := client.ReadMessage()
				require.NoError(t, err)
				require.Contains(t, string(msg), "conflicting realtime response usage")
			}
			_ = client.Close()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("accounting did not finish")
			}
			searchQuotaBalances(t, db, false, tc.spent, 1)
		})
	}
}
