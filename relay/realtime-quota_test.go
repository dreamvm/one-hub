package relay_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"one-api/common/requester"
	"one-api/providers/openai"

	"one-api/common/config"
	"one-api/model"
	"one-api/relay"
)

func TestRealtimeQuotaRejectsBeforeUpstream(t *testing.T) {
	for _, balance := range []string{"user", "token"} {
		t.Run(balance, func(t *testing.T) {
			db, c, _, _ := searchQuotaFixture(t, false)
			model.PricingInstance.Prices["query-realtime"] = model.PricingInstance.Prices["query-fixture"]
			if balance == "user" {
				require.NoError(t, db.Model(&model.User{}).Where("id=1").Update("quota", 19).Error)
			} else {
				require.NoError(t, db.Model(&model.Token{}).Where("id=1").Update("remain_quota", 19).Error)
			}
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()
				_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"session.created"}`))
			}))
			defer upstream.Close()
			require.NoError(t, db.Model(&model.Channel{}).Where("id=1").Updates(map[string]any{"type": config.ChannelTypeOpenAI, "status": config.ChannelStatusEnabled, "base_url": upstream.URL, "proxy": "", "key": "fixture-only"}).Error)
			c.Set("specific_channel_id", 1)
			done := make(chan struct{})
			router := gin.New()
			router.GET("/realtime", func(ctx *gin.Context) {
				defer close(done)
				for key, value := range c.Keys {
					ctx.Set(key, value)
				}
				relay.ChatRealtime(ctx)
			})
			gateway := httptest.NewServer(router)
			defer gateway.Close()
			client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(gateway.URL, "http")+"/realtime?model=query-realtime", nil)
			require.NoError(t, err)
			defer client.Close()
			_ = client.SetReadDeadline(time.Now().Add(3 * time.Second))
			_, _, _ = client.ReadMessage()
			_ = client.Close()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("handler did not finish")
			}
			require.Zero(t, calls.Load(), "an underfunded session cannot open the billable upstream")
		})
	}
}

func TestRealtimeQuotaReportedUsageAndFailedHandshake(t *testing.T) {
	for _, mode := range []string{"normal", "token exhaustion", "handshake error", "invalid first frame", "no first frame", "initial usage", "retry"} {
		t.Run(mode, func(t *testing.T) {
			db, c, _, _ := searchQuotaFixture(t, false)
			model.PricingInstance.Prices["query-realtime"] = model.PricingInstance.Prices["query-fixture"]
			oldTimeout := viper.Get("connect_timeout")
			viper.Set("connect_timeout", 1)
			t.Cleanup(func() { viper.Set("connect_timeout", oldTimeout) })
			oldRetry := config.RetryTimes
			config.RetryTimes = 1
			if mode == "retry" {
				config.RetryTimes = 2
			}
			t.Cleanup(func() { config.RetryTimes = oldRetry })
			if mode == "token exhaustion" {
				require.NoError(t, db.Model(&model.Token{}).Where("id=1").Update("remain_quota", 25).Error)
			}
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := calls.Add(1)
				if mode == "handshake error" || mode == "retry" && n == 1 {
					w.WriteHeader(http.StatusBadGateway)
					return
				}
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()
				_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
				if mode == "no first frame" {
					_, _, _ = conn.ReadMessage()
					return
				}
				if mode == "initial usage" {
					_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.done","response":{"usage":{"input_tokens":7,"total_tokens":7}}}`))
					_, _, _ = conn.ReadMessage()
					return
				}
				if mode == "invalid first frame" {
					_ = conn.WriteMessage(websocket.BinaryMessage, []byte("invalid"))
					return
				}
				_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"session.created"}`))
				// Wait for the gateway to start normal transfer before reporting usage.
				if _, _, err = conn.ReadMessage(); err != nil {
					return
				}
				_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.done","response":{"usage":{"input_tokens":7,"total_tokens":7}}}`))
				_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.done","response":{"usage":{"input_tokens":14,"total_tokens":14}}}`))
				_, _, _ = conn.ReadMessage()
			}))
			defer upstream.Close()
			require.NoError(t, db.Model(&model.Channel{}).Where("id=1").Updates(map[string]any{"type": config.ChannelTypeOpenAI, "status": config.ChannelStatusEnabled, "base_url": upstream.URL, "proxy": "", "key": "fixture-only"}).Error)
			c.Set("specific_channel_id", 1)
			done := make(chan struct{})
			router := gin.New()
			router.GET("/realtime", func(ctx *gin.Context) {
				defer close(done)
				for key, value := range c.Keys {
					ctx.Set(key, value)
				}
				relay.ChatRealtime(ctx)
			})
			gateway := httptest.NewServer(router)
			defer gateway.Close()
			client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(gateway.URL, "http")+"/realtime?model=query-realtime", nil)
			require.NoError(t, err)
			defer client.Close()
			_ = client.SetReadDeadline(time.Now().Add(3 * time.Second))
			_, first, err := client.ReadMessage()
			require.NoError(t, err)
			spent, records := 0, 0
			if mode == "normal" || mode == "token exhaustion" || mode == "retry" {
				require.Contains(t, string(first), "session.created")
				require.NoError(t, client.WriteMessage(websocket.TextMessage, []byte(`{"type":"fixture.start"}`)))
				for i := 0; i < 2; i++ {
					_, msg, err := client.ReadMessage()
					require.NoError(t, err)
					require.Contains(t, string(msg), "response.done")
				}
				spent, records = 21, 1
			} else if mode == "initial usage" {
				require.Contains(t, string(first), "response.done")
				spent, records = 7, 1
			} else {
				require.Contains(t, string(first), "error")
			}
			_ = client.Close()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("handler did not complete accounting")
			}
			expectedCalls := int32(1)
			if mode == "retry" {
				expectedCalls = 2
			}
			require.Equal(t, expectedCalls, calls.Load())
			if mode == "token exhaustion" {
				var token model.Token
				require.NoError(t, db.First(&token, 1).Error)
				require.Equal(t, 4, token.RemainQuota)
				// Normalize only the initial balance for the shared fee assertion.
				require.NoError(t, db.Model(&token).Update("remain_quota", 979).Error)
			}
			searchQuotaBalances(t, db, false, spent, records)
		})
	}
}

func TestRealtimeInvalidResponseDoneIsRejected(t *testing.T) {
	p := &openai.OpenAIProvider{}
	for _, msg := range []string{`{"type":"response.done"}`, `{"type":"response.done","response":null}`} {
		require.NotPanics(t, func() {
			accepted, _, _, err := p.HandleMessage(requester.SupplierMessage, websocket.TextMessage, []byte(msg))
			require.False(t, accepted)
			require.Error(t, err)
		})
	}
}
