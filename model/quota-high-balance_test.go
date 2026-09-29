package model_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"one-api/common/config"
	"one-api/model"
	mjprovider "one-api/providers/midjourney"
	"one-api/relay/midjourney"
	"one-api/relay/relay_util"
	"one-api/relay/task"
	"one-api/types"
)

func TestQuotaHighBalanceAdmission(t *testing.T) {
	for _, redisEnabled := range []bool{false, true} {
		for _, batch := range []bool{false, true} {
			for _, balance := range []int{2000, 2001, 10000} {
				t.Run(fmt.Sprintf("redis=%t/batch=%t/user=%d", redisEnabled, batch, balance), func(t *testing.T) {
					db, c := quotaLifecycleFixture(t, batch, false, redisEnabled)
					require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("quota", balance).Error)
					require.NoError(t, db.Model(&model.Token{}).Where("id = ?", c.GetInt("token_id")).Update("remain_quota", 19).Error)
					q := relay_util.NewQuota(c, "quota-fixture", 0)
					err := q.PreQuotaConsumption()
					require.NotNil(t, err)
					require.Equal(t, http.StatusForbidden, err.StatusCode)
					require.Equal(t, "pre_consume_token_quota_failed", err.Code)
					require.False(t, q.HandelStatus)
					q.Undo(c)
					model.FlushQuotaBatchForTest()
					requireQuotaPair(t, db, c.GetInt("token_id"), balance, 19, 0)
				})
			}
		}
	}
}

func TestQuotaHighBalanceStaleUserCache(t *testing.T) {
	for _, unlimited := range []bool{false, true} {
		t.Run(fmt.Sprint(unlimited), func(t *testing.T) {
			db, c := quotaLifecycleFixture(t, true, unlimited, true)
			require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("quota", 10000).Error)
			seedUserQuotaCache(t, "10000")
			require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("quota", 19).Error)
			q := relay_util.NewQuota(c, "quota-fixture", 0)
			require.NotNil(t, q.PreQuotaConsumption(), "a high cached balance cannot skip the database reservation")
			require.False(t, q.HandelStatus)
			requireQuotaPair(t, db, c.GetInt("token_id"), 19, 1000, 0)
		})
	}
}

func TestQuotaHighBalanceReservation(t *testing.T) {
	for _, unlimited := range []bool{false, true} {
		for _, batch := range []bool{false, true} {
			for _, pricing := range []string{"tokens", "times", "free"} {
				t.Run(fmt.Sprintf("unlimited=%t/batch=%t/%s", unlimited, batch, pricing), func(t *testing.T) {
					db, c := quotaLifecycleFixture(t, batch, unlimited, true)
					require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("quota", 10000).Error)
					initial := 20
					if unlimited {
						initial = 0
					}
					require.NoError(t, db.Model(&model.Token{}).Where("id = ?", c.GetInt("token_id")).Update("remain_quota", initial).Error)
					name, amount := "quota-fixture", 20
					if pricing == "times" {
						model.PricingInstance.Prices[name] = &model.Price{Type: model.TimesPriceType, Input: .02}
					} else if pricing == "free" {
						name, amount = "quota-free", 0
					}
					q := relay_util.NewQuota(c, name, 0)
					require.Nil(t, q.PreQuotaConsumption())
					require.Equal(t, amount > 0, q.HandelStatus)
					tokenSpent := amount
					if unlimited {
						tokenSpent = 0
					}
					requireQuotaPair(t, db, c.GetInt("token_id"), 10000-amount, initial-tokenSpent, tokenSpent)
					model.FlushQuotaBatchForTest()
					requireQuotaPair(t, db, c.GetInt("token_id"), 10000-amount, initial-tokenSpent, tokenSpent)
					q.Undo(c)
					q.Undo(c)
					requireQuotaPair(t, db, c.GetInt("token_id"), 10000, initial, 0)
				})
			}
		}
	}
}

// This control passes both before and after removal of the shortcut: the final
// fee is unchanged even though a successful high-balance request now reserves.
func TestQuotaHighBalanceSettlementControl(t *testing.T) {
	for _, unlimited := range []bool{false, true} {
		for _, amount := range []int{0, 7, 20, 35} {
			t.Run(fmt.Sprintf("unlimited=%t/amount=%d", unlimited, amount), func(t *testing.T) {
				db, c := quotaLifecycleFixture(t, true, unlimited, true)
				require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("quota", 10000).Error)
				q := relay_util.NewQuota(c, "quota-fixture", 0)
				require.Nil(t, q.PreQuotaConsumption())
				q.Consume(c, &types.Usage{PromptTokens: amount}, false)
				q.Undo(c)
				model.FlushQuotaBatchForTest()
				require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("quota", gorm.Expr("quota - 9000")).Error)
				quotaBalances(t, db, c, amount, 1)
			})
		}
	}
}

func TestQuotaHighBalanceConcurrentReservations(t *testing.T) {
	db, c := quotaLifecycleFixture(t, true, false, true)
	require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("quota", 10000).Error)
	require.NoError(t, db.Model(&model.Token{}).Where("id = ?", c.GetInt("token_id")).Update("remain_quota", 95).Error)
	// A legacy high balance snapshot must not override database reservations.
	seedUserQuotaCache(t, "10000")
	var accepted atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 24; i++ {
		q := relay_util.NewQuota(c, "quota-fixture", 0)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if q.PreQuotaConsumption() == nil {
				accepted.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	require.EqualValues(t, 4, accepted.Load())
	requireQuotaPair(t, db, c.GetInt("token_id"), 9920, 15, 80)
	model.FlushQuotaBatchForTest()
	requireQuotaPair(t, db, c.GetInt("token_id"), 9920, 15, 80)
}

func TestQuotaHighBalanceRejectsBeforeTaskUpstream(t *testing.T) {
	for _, caller := range []string{"task", "mj", "mj-swap"} {
		t.Run(caller, func(t *testing.T) {
			db, c := quotaLifecycleFixture(t, false, false)
			require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("quota", 10000).Error)
			require.NoError(t, db.Model(&model.Token{}).Where("id = ?", c.GetInt("token_id")).Update("remain_quota", 19).Error)
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
			defer server.Close()
			if caller == "task" {
				quotaChannel(t, db, c, server.URL, config.ChannelTypeSuno, "suno_lyrics")
				model.PricingInstance.Prices["suno_lyrics"] = &model.Price{Type: model.TimesPriceType, Input: .02}
				r := gin.New()
				r.POST("/suno/submit/:action", func(requestContext *gin.Context) {
					for key, value := range c.Keys {
						requestContext.Set(key, value)
					}
					task.RelayTaskSubmit(requestContext)
				})
				request := httptest.NewRequest("POST", "/suno/submit/lyrics", strings.NewReader(`{"prompt":"fixture"}`))
				request.Header.Set("Content-Type", "application/json")
				r.ServeHTTP(httptest.NewRecorder(), request)
			} else {
				action, route, body := mjprovider.MjActionImagine, "/mj/submit/imagine", `{"prompt":"fixture"}`
				if caller == "mj-swap" {
					action, route, body = mjprovider.MjActionSwapFace, "/mj/insight-face/swap", `{"sourceBase64":"fixture","targetBase64":"fixture"}`
				}
				name := midjourney.CoverActionToModelName(action, "fast")
				model.PricingInstance.Prices[name] = &model.Price{Type: model.TimesPriceType, Input: .02}
				quotaChannel(t, db, c, server.URL, config.ChannelTypeMidjourney, name)
				c.Set("mj_model", "fast")
				c.Request = httptest.NewRequest("POST", route, strings.NewReader(body))
				c.Request.Header.Set("Content-Type", "application/json")
				if caller == "mj-swap" {
					midjourney.RelaySwapFace(c)
				} else {
					midjourney.RelayMidjourneySubmit(c, mjprovider.RelayModeMidjourneyImagine)
				}
			}
			require.Zero(t, calls.Load(), "insufficient finite token quota must stop upstream submission")
			requireQuotaPair(t, db, c.GetInt("token_id"), 10000, 19, 0)
		})
	}
}
