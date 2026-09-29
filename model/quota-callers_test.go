package model_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"one-api/common/config"
	"one-api/common/requester"
	"one-api/controller"
	"one-api/model"
	mjprovider "one-api/providers/midjourney"
	"one-api/relay/midjourney"
	"one-api/relay/task"
)

func quotaChannel(t *testing.T, db *gorm.DB, c *gin.Context, url string, channelType int, models string) {
	t.Helper()
	oldClient := requester.HTTPClient
	requester.HTTPClient = &http.Client{Timeout: time.Second}
	t.Cleanup(func() { requester.HTTPClient = oldClient })
	oldChannels, oldRule, oldMatch, oldModelGroup := model.ChannelGroup.Channels, model.ChannelGroup.Rule, model.ChannelGroup.Match, model.ChannelGroup.ModelGroup
	oldRetry, oldCooldown := config.RetryTimes, config.RetryCooldownSeconds
	config.RetryTimes, config.RetryCooldownSeconds = 2, 0
	t.Cleanup(func() {
		task.DeactivateTask()
		controller.DeactivateMidjourneyTaskBulk()
		model.ChannelGroup.Channels, model.ChannelGroup.Rule, model.ChannelGroup.Match, model.ChannelGroup.ModelGroup = oldChannels, oldRule, oldMatch, oldModelGroup
		config.RetryTimes, config.RetryCooldownSeconds = oldRetry, oldCooldown
	})
	require.NoError(t, db.Model(&model.Channel{}).Where("id=1").Updates(map[string]any{
		"type": channelType, "base_url": url, "status": config.ChannelStatusEnabled, "group": "default", "models": models, "key": "fixture-only", "weight": 1, "proxy": "",
	}).Error)
	model.ChannelGroup.Load()
	c.Set("token_group", "default")
}

func TestQuotaTaskRetryLifecycle(t *testing.T) {
	for _, mode := range []string{"first success", "retry success", "all fail", "provider missing", "no retry"} {
		t.Run(mode, func(t *testing.T) {
			db, c := quotaLifecycleFixture(t, false, false)
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				if mode == "first success" || mode == "retry success" && n == 2 {
					_, _ = w.Write([]byte(`{"code":"success","data":"task-fixture"}`))
					return
				}
				if mode == "provider missing" {
					model.ChannelGroup.Disable(1)
				}
				w.WriteHeader(500)
				_, _ = w.Write([]byte(`{"code":"fixture_error","message":"try again"}`))
			}))
			defer server.Close()
			quotaChannel(t, db, c, server.URL, config.ChannelTypeSuno, "suno_lyrics")
			model.PricingInstance.Prices["suno_lyrics"] = &model.Price{Type: model.TimesPriceType, Input: .02}
			if mode == "no retry" {
				c.Set("specific_channel_id", 1)
			}
			w := httptest.NewRecorder()
			r := gin.New()
			r.POST("/suno/submit/:action", func(requestContext *gin.Context) {
				for key, value := range c.Keys {
					requestContext.Set(key, value)
				}
				task.RelayTaskSubmit(requestContext)
			})
			req := httptest.NewRequest("POST", "/suno/submit/lyrics", strings.NewReader(`{"prompt":"fixture"}`))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			spent, requests, count := 0, 0, int32(1)
			if mode == "first success" || mode == "retry success" {
				spent, requests = 20, 1
				require.Equal(t, 200, w.Code)
				require.Contains(t, w.Body.String(), "task-fixture", "successful retries must return the same response contract")
			}
			if mode == "retry success" {
				count = 2
			}
			if mode == "all fail" {
				count = 3
			}
			require.Equal(t, count, calls.Load())
			quotaBalances(t, db, c, spent, requests)
			var rows int64
			require.NoError(t, db.Model(&model.Task{}).Count(&rows).Error)
			require.EqualValues(t, requests, rows)
		})
	}
}

func TestQuotaMJModeSwitchLifecycle(t *testing.T) {
	for _, swap := range []bool{false, true} {
		for _, outcome := range []string{"success", "quota denied", "network failure", "upstream rejection"} {
			t.Run(fmt.Sprintf("swap=%t/%s", swap, outcome), func(t *testing.T) {
				db, c := quotaLifecycleFixture(t, false, false)
				require.NoError(t, db.AutoMigrate(&model.Midjourney{}))
				var calls atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					n := calls.Add(1)
					if n == 1 {
						_, _ = w.Write([]byte(`{"code":3,"description":"no fast capacity"}`))
						return
					}
					if outcome == "network failure" {
						connection, _, err := w.(http.Hijacker).Hijack()
						if err == nil {
							_ = connection.Close()
						}
						return
					}
					if outcome == "upstream rejection" {
						_, _ = w.Write([]byte(`{"code":4,"description":"rejected"}`))
						return
					}
					_, _ = w.Write([]byte(`{"code":1,"result":"mj-fixture"}`))
				}))
				defer server.Close()
				action, route, body := mjprovider.MjActionImagine, "/mj/submit/imagine", `{"prompt":"fixture"}`
				if swap {
					action, route, body = mjprovider.MjActionSwapFace, "/mj/insight-face/swap", `{"sourceBase64":"fixture","targetBase64":"fixture"}`
				}
				fast, relax := midjourney.CoverActionToModelName(action, "fast"), midjourney.CoverActionToModelName(action, "relax")
				model.PricingInstance.Prices[fast] = &model.Price{Type: model.TimesPriceType, Input: .02}
				relaxPrice := .03
				if outcome == "quota denied" {
					relaxPrice = 2
				}
				model.PricingInstance.Prices[relax] = &model.Price{Type: model.TimesPriceType, Input: relaxPrice}
				quotaChannel(t, db, c, server.URL, config.ChannelTypeMidjourney, fast+","+relax)
				c.Set("mj_model", "fast")
				c.Request = httptest.NewRequest("POST", route, strings.NewReader(body))
				c.Request.Header.Set("Content-Type", "application/json")
				require.NotPanics(t, func() {
					if swap {
						midjourney.RelaySwapFace(c)
					} else {
						midjourney.RelayMidjourneySubmit(c, mjprovider.RelayModeMidjourneyImagine)
					}
				})
				count := int32(2)
				if outcome == "quota denied" {
					count = 1
				}
				require.Equal(t, count, calls.Load())
				spent, requests := 0, 0
				if outcome == "success" {
					spent, requests = 30, 1
				}
				quotaBalances(t, db, c, spent, requests)
			})
		}
	}
}

func TestQuotaMJLegitimateBilling(t *testing.T) {
	for _, action := range []string{mjprovider.MjActionImagine, mjprovider.MjActionInPaint, mjprovider.MjActionCustomZoom} {
		for _, code := range []int{1, 21, 22} {
			t.Run(fmt.Sprintf("%s/%d", action, code), func(t *testing.T) {
				db, c := quotaLifecycleFixture(t, false, false)
				require.NoError(t, db.AutoMigrate(&model.Midjourney{}))
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = fmt.Fprintf(w, `{"code":%d,"result":"mj-new-fixture"}`, code)
				}))
				defer server.Close()
				name := midjourney.CoverActionToModelName(action, "fast")
				model.PricingInstance.Prices[name] = &model.Price{Type: model.TimesPriceType, Input: .02}
				quotaChannel(t, db, c, server.URL, config.ChannelTypeMidjourney, name)
				c.Set("mj_model", "fast")
				mode := mjprovider.RelayModeMidjourneyImagine
				body := `{"prompt":"fixture"}`
				if action != mjprovider.MjActionImagine {
					mode = mjprovider.RelayModeMidjourneyChange
					require.NoError(t, db.Create(&model.Midjourney{UserId: 1, MjId: "mj-origin", ChannelId: 1, Status: "SUCCESS", Mode: "fast"}).Error)
					body = fmt.Sprintf(`{"taskId":"mj-origin","action":%q,"index":1}`, action)
				}
				c.Request = httptest.NewRequest("POST", "/mj/submit/change", strings.NewReader(body))
				c.Request.Header.Set("Content-Type", "application/json")
				require.Nil(t, midjourney.RelayMidjourneySubmit(c, mode))
				spent, requests := 20, 1
				if action != mjprovider.MjActionImagine {
					spent, requests = 0, 0
				}
				quotaBalances(t, db, c, spent, requests)
			})
		}
	}
}
