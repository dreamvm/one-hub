package relay_test

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"one-api/common/config"
	"one-api/model"
	"one-api/relay"
)

// Only the mock upstream can enable this fault, after the gateway has already
// sent HTTP 101. No timing-dependent TCP reset or production test hook is used.
type realtimeFailWriteConn struct {
	net.Conn
	fail *atomic.Bool
}

func (c realtimeFailWriteConn) Write(p []byte) (int, error) {
	if c.fail.Load() {
		return 0, errors.New("fixture downstream write failure")
	}
	return c.Conn.Write(p)
}

type realtimeFailWriteResponse struct {
	http.ResponseWriter
	fail *atomic.Bool
}

func (w realtimeFailWriteResponse) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	c, rw, err := w.ResponseWriter.(http.Hijacker).Hijack()
	if err != nil {
		return nil, nil, err
	}
	return realtimeFailWriteConn{Conn: c, fail: w.fail}, rw, nil
}

func TestRealtimeFirstUsageFailureSettlement(t *testing.T) {
	for _, tc := range []struct {
		name, usage, pricing             string
		writeFailure, limited, unlimited bool
		spent, records, calls            int
	}{
		{"audio topup", `{"input_token_details":{"audio_tokens":25}}`, "tokens", false, true, false, 25, 1, 1},
		{"audio user topup unlimited token", `{"output_token_details":{"audio_tokens":25}}`, "tokens", false, true, true, 25, 1, 1},
		{"audio write", `{"input_token_details":{"audio_tokens":7}}`, "tokens", true, false, false, 7, 1, 1},
		{"reasoning write", `{"output_token_details":{"reasoning_tokens":7}}`, "tokens", true, false, false, 7, 1, 1},
		{"fixed price audio write", `{"output_token_details":{"audio_tokens":7}}`, "times", true, false, false, 20, 1, 1},
		{"free audio write", `{"input_token_details":{"audio_tokens":7}}`, "free", true, false, false, 0, 1, 1},
		{"zero group audio write", `{"input_token_details":{"audio_tokens":7}}`, "zero group", true, false, false, 0, 1, 1},
		{"total only write", `{"total_tokens":7}`, "tokens", true, false, false, 0, 1, 1},
		{"unpriceable audio preserves reservation", `{"input_token_details":{"audio_tokens":7}}`, "invalid computation", false, false, false, 20, 0, 1},
		{"ordinary tokens write", `{"input_tokens":7,"total_tokens":7}`, "tokens", true, false, false, 7, 1, 1},
		{"zero usage write retries", `{}`, "tokens", true, false, false, 0, 0, 2},
		{"invalid usage preserves reservation", `{"input_token_details":{"audio_tokens":-7}}`, "tokens", false, false, false, 20, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, fixture, _, _ := searchQuotaFixture(t, tc.unlimited)
			price := model.PricingInstance.Prices["query-fixture"]
			ratios := datatypes.NewJSONType(map[string]float64{config.UsageExtraInputAudio: 2, config.UsageExtraOutputAudio: 2, config.UsageExtraReasoning: 2})
			price.ExtraRatios = &ratios
			switch tc.pricing {
			case "times":
				price.Type, price.Input = model.TimesPriceType, .02
			case "free":
				price.Input, price.Output = 0, 0
			case "zero group":
				fixture.Set("group_ratio", float64(0))
			case "invalid computation":
				ratios = datatypes.NewJSONType(map[string]float64{config.UsageExtraInputAudio: .5})
			}
			model.PricingInstance.Prices["query-realtime"] = price
			if tc.limited {
				if tc.unlimited {
					require.NoError(t, db.Model(&model.User{}).Where("id=1").Update("quota", 25).Error)
				} else {
					require.NoError(t, db.Model(&model.Token{}).Where("id=1").Update("remain_quota", 25).Error)
				}
			}
			oldRetry := config.RetryTimes
			config.RetryTimes = 2
			t.Cleanup(func() { config.RetryTimes = oldRetry })
			var calls atomic.Int32
			var fail atomic.Bool
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
				_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.done","response":{"id":"fixture-first","usage":`+tc.usage+`}}`))
				_, _, _ = conn.ReadMessage()
			}))
			t.Cleanup(upstream.Close)
			require.NoError(t, db.Model(&model.Channel{}).Where("id=1").Updates(map[string]any{"type": config.ChannelTypeOpenAI, "status": config.ChannelStatusEnabled, "base_url": upstream.URL, "proxy": "", "key": "fixture-only"}).Error)
			fixture.Set("specific_channel_id", 1)
			done := make(chan struct{})
			router := gin.New()
			router.GET("/realtime", func(ctx *gin.Context) {
				defer close(done)
				for key, value := range fixture.Keys {
					ctx.Set(key, value)
				}
				relay.ChatRealtime(ctx)
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
					t.Error("handler did not stop")
				}
			})
			_ = client.SetReadDeadline(time.Now().Add(3 * time.Second))
			_, _, _ = client.ReadMessage()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("accounting did not finish")
			}
			require.EqualValues(t, tc.calls, calls.Load())
			if tc.limited {
				if tc.unlimited {
					var user model.User
					require.NoError(t, db.First(&user, 1).Error)
					require.Equal(t, 25-tc.spent, user.Quota)
					require.NoError(t, db.Model(&user).Update("quota", 1000-tc.spent).Error)
				} else {
					var token model.Token
					require.NoError(t, db.First(&token, 1).Error)
					require.Equal(t, 25-tc.spent, token.RemainQuota)
					require.NoError(t, db.Model(&token).Update("remain_quota", 1000-tc.spent).Error)
				}
			}
			searchQuotaBalances(t, db, tc.unlimited, tc.spent, tc.records)
			var receipts []model.QuotaReservation
			require.NoError(t, db.Find(&receipts).Error)
			require.Len(t, receipts, tc.calls)
			for _, receipt := range receipts {
				if tc.records > 0 {
					require.Equal(t, model.QuotaReservationConsumed, receipt.State)
					require.Equal(t, tc.spent, receipt.FinalQuota)
				} else if tc.pricing == "invalid computation" {
					require.Equal(t, model.QuotaReservationReserved, receipt.State)
					require.Empty(t, receipt.Outcome)
					require.Equal(t, 20, receipt.ReservedQuota)
				} else if tc.name == "invalid usage preserves reservation" {
					require.Equal(t, model.QuotaReservationReconcile, receipt.State)
					require.Empty(t, receipt.Outcome)
					require.Equal(t, 20, receipt.ReservedQuota)
				} else {
					require.Equal(t, model.QuotaReservationRefunded, receipt.State)
				}
			}
		})
	}
}
