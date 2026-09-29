package model_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"one-api/common/config"
	"one-api/relay/task"

	"github.com/stretchr/testify/require"

	"one-api/common/requester"
	"one-api/controller"
	"one-api/model"
	mjprovider "one-api/providers/midjourney"
	"one-api/relay/relay_util"
	"one-api/types"
)

func TestQuotaBackgroundMJRefundPairAndReplay(t *testing.T) {
	for _, outcome := range []string{"FAILURE", "SUCCESS"} {
		t.Run(outcome, func(t *testing.T) {
			db, c := quotaLifecycleFixture(t, false, false)
			oldClient := requester.HTTPClient
			requester.HTTPClient = &http.Client{Timeout: time.Second}
			t.Cleanup(func() { requester.HTTPClient = oldClient })
			require.NoError(t, db.AutoMigrate(&model.Midjourney{}))
			q := relay_util.NewQuota(c, "quota-fixture", 0)
			require.Nil(t, q.PreQuotaConsumption())
			q.Consume(c, &types.Usage{PromptTokens: 20}, false)
			var receipt model.QuotaReservation
			require.NoError(t, db.First(&receipt).Error)
			now := time.Now().UnixMilli()
			original := model.Midjourney{UserId: 1, TokenID: c.GetInt("token_id"), ChannelId: 1, Quota: 20, MjId: "refund-fixture", SubmitTime: now, Progress: "10%", Status: "IN_PROGRESS"}
			require.NoError(t, original.Insert())
			require.NoError(t, db.Model(&original).Update("reservation_id", receipt.ID).Error)
			require.NoError(t, db.First(&original, original.Id).Error)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reason := ""
				if outcome == "FAILURE" {
					reason = "fixture failed"
				}
				require.NoError(t, json.NewEncoder(w).Encode([]mjprovider.MidjourneyDto{{MjId: original.MjId, SubmitTime: now, Progress: "100%", Status: outcome, FailReason: reason}}))
			}))
			defer server.Close()
			channel := &model.Channel{Id: 1, BaseURL: &server.URL}
			for i := 0; i < 2; i++ {
				stale := original
				require.NoError(t, controller.MjTaskHandler(channel, []string{original.MjId}, map[string]*model.Midjourney{original.MjId: &stale}))
				expected := 980
				if outcome == "FAILURE" {
					expected = 1000
				}
				requireQuotaPair(t, db, original.TokenID, expected, expected, 1000-expected)
			}
		})
	}
}

func TestQuotaBackgroundTaskSubmitPoll(t *testing.T) {
	for _, platform := range []string{model.TaskPlatformSuno, model.TaskPlatformKling} {
		for _, unlimited := range []bool{false, true} {
			for _, outcome := range []string{"failure", "success", "processing", "unbound", "transport"} {
				t.Run(fmt.Sprintf("%s/unlimited=%t/%s", platform, unlimited, outcome), func(t *testing.T) {
					db, c := quotaLifecycleFixture(t, true, unlimited)
					name, channelType, route, path := "suno_lyrics", config.ChannelTypeSuno, "/suno/submit/:action", "/suno/submit/lyrics"
					if platform == model.TaskPlatformKling {
						name, channelType, route, path = "kling-video_kling-v1_std_5", config.ChannelTypeKling, "/kling/v1/:class/:action", "/kling/v1/videos/text2video"
					}
					var polls atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						if r.Method == "POST" && !strings.Contains(r.URL.Path, "fetch") {
							if platform == model.TaskPlatformSuno {
								_, _ = w.Write([]byte(`{"code":"success","data":"provider-task"}`))
							} else {
								_, _ = w.Write([]byte(`{"code":0,"data":{"task_id":"provider-task","task_status":"submitted"}}`))
							}
							return
						}
						polls.Add(1)
						if outcome == "transport" {
							w.WriteHeader(503)
							_, _ = w.Write([]byte(`{"code":503,"message":"fixture unavailable"}`))
							return
						}
						id := "provider-task"
						if outcome == "unbound" {
							id = "other-task"
						}
						if platform == model.TaskPlatformSuno {
							status := "IN_PROGRESS"
							reason := ""
							if outcome == "failure" || outcome == "unbound" {
								status, reason = "FAILURE", "fixture failure"
							}
							if outcome == "success" {
								status = "SUCCESS"
							}
							require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"code": "success", "data": []map[string]any{{"task_id": id, "status": status, "fail_reason": reason, "data": map[string]any{}}}}))
						} else {
							status := "processing"
							if outcome == "failure" || outcome == "unbound" {
								status = "failed"
							}
							if outcome == "success" {
								status = "succeed"
							}
							// task_status_msg can describe normal progress; it is not a failure flag.
							require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"task_id": id, "task_status": status, "task_status_msg": "fixture status description"}}))
						}
					}))
					defer server.Close()
					quotaChannel(t, db, c, server.URL, channelType, name)
					model.PricingInstance.Prices[name] = &model.Price{Type: model.TimesPriceType, Input: .02}
					router := gin.New()
					router.POST(route, func(ctx *gin.Context) {
						for k, v := range c.Keys {
							ctx.Set(k, v)
						}
						task.RelayTaskSubmit(ctx)
					})
					response := httptest.NewRecorder()
					request := httptest.NewRequest("POST", path, strings.NewReader(`{"prompt":"fixture"}`))
					request.Header.Set("Content-Type", "application/json")
					router.ServeHTTP(response, request)
					require.Equal(t, 200, response.Code, response.Body.String())
					var original model.Task
					require.NoError(t, db.First(&original).Error)
					require.Equal(t, platform, original.Platform)
					require.NotEmpty(t, original.ReservationID)
					var receipt model.QuotaReservation
					require.NoError(t, db.First(&receipt, "id = ?", original.ReservationID).Error)
					require.Equal(t, model.QuotaReservationConsumed, receipt.State)
					for i := 0; i < 2; i++ {
						stale := original
						task.UpdateTaskByPlatform(context.Background(), platform, map[int][]string{1: {original.TaskID}}, map[string]*model.Task{original.TaskID: &stale})
					}
					require.EqualValues(t, 2, polls.Load(), "normal lowercase Kling tasks must actually poll")
					expected := 980
					tokenExpected, tokenUsed := 980, 20
					if unlimited {
						tokenExpected, tokenUsed = 1000, 0
					}
					if outcome == "failure" {
						expected, tokenExpected, tokenUsed = 1000, 1000, 0
					}
					model.FlushQuotaBatchForTest()
					requireQuotaPair(t, db, original.TokenID, expected, tokenExpected, tokenUsed)
					var saved model.Task
					require.NoError(t, db.First(&saved, original.ID).Error)
					if outcome == "failure" || outcome == "success" {
						require.Equal(t, 100, saved.Progress)
					} else {
						require.NotEqual(t, 100, saved.Progress)
					}
					var refundLogs int64
					require.NoError(t, db.Model(&model.Log{}).Where("type = ?", model.LogTypeSystem).Count(&refundLogs).Error)
					if outcome == "failure" {
						require.EqualValues(t, 1, refundLogs)
					} else {
						require.Zero(t, refundLogs)
					}
				})
			}
		}
	}
}

func TestQuotaBackgroundSunoCrossChannelRetry(t *testing.T) {
	db, c := quotaLifecycleFixture(t, false, false)
	var submits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "submit") {
			if submits.Add(1) == 1 {
				model.ChannelGroup.Disable(1)
				w.WriteHeader(500)
				_, _ = w.Write([]byte(`{"code":"fixture","message":"retry"}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":"success","data":"retry-task"}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":"success","data":[{"task_id":"retry-task","status":"FAILURE","fail_reason":"fixture failure","data":{}}]}`))
	}))
	defer server.Close()
	quotaChannel(t, db, c, server.URL, config.ChannelTypeSuno, "suno_lyrics")
	// Initially only A is eligible; its failed response disables it. B remains
	// available at a lower priority, guaranteeing the actual retry changes channel.
	require.NoError(t, db.Model(&model.Channel{}).Where("id = 1").Update("priority", 10).Error)
	var second model.Channel
	require.NoError(t, db.First(&second, 1).Error)
	second.Id = 2
	priority := int64(0)
	second.Priority = &priority
	require.NoError(t, db.Create(&second).Error)
	model.ChannelGroup.Load()
	model.PricingInstance.Prices["suno_lyrics"] = &model.Price{Type: model.TimesPriceType, Input: .02}
	router := gin.New()
	router.POST("/suno/submit/:action", func(ctx *gin.Context) {
		for k, v := range c.Keys {
			ctx.Set(k, v)
		}
		task.RelayTaskSubmit(ctx)
	})
	response := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/suno/submit/lyrics", strings.NewReader(`{"prompt":"fixture"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	require.Equal(t, 200, response.Code, response.Body.String())
	require.EqualValues(t, 2, submits.Load())
	var saved model.Task
	require.NoError(t, db.First(&saved).Error)
	require.Equal(t, 2, saved.ChannelId)
	requireQuotaPair(t, db, saved.TokenID, 980, 980, 20)
	task.UpdateTaskByPlatform(context.Background(), model.TaskPlatformSuno, map[int][]string{2: {saved.TaskID}}, map[string]*model.Task{saved.TaskID: &saved})
	require.NoError(t, model.RecoverQuotaReservations(context.Background(), 100))
	requireQuotaPair(t, db, saved.TokenID, 1000, 1000, 0)
}
