package model_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	redisclient "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"one-api/common/redis"
	"one-api/common/requester"
	"one-api/controller"
	"one-api/model"
	mjprovider "one-api/providers/midjourney"
	"one-api/relay/relay_util"
	"one-api/types"
)

func seedUserQuotaCache(t *testing.T, value string) {
	t.Helper()
	require.NoError(t, redis.RedisSet(fmt.Sprintf(model.UserQuotaCacheKey, 1), value, time.Hour))
}

func unavailableQuotaRedis(t *testing.T) {
	t.Helper()
	old := redis.RDB
	client := redisclient.NewClient(&redisclient.Options{Addr: "fixture.invalid:6379"})
	require.NoError(t, client.Close())
	redis.RDB = client
	t.Cleanup(func() { redis.RDB = old })
}

func TestQuotaCacheReadsCommittedBalance(t *testing.T) {
	for _, cached := range []string{"0", "-20", "100000", "invalid", "99999999999999999999999999999", "missing"} {
		t.Run(cached, func(t *testing.T) {
			db, _ := quotaLifecycleFixture(t, false, false, true)
			if cached != "missing" {
				seedUserQuotaCache(t, cached)
			}
			balance, err := model.CacheGetUserQuota(1)
			require.NoError(t, err)
			require.Equal(t, 1000, balance)
			require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("quota", 7).Error)
			balance, err = model.CacheGetUserQuota(1)
			require.NoError(t, err)
			require.Equal(t, 7, balance, "a stale fill cannot restore old authorization")
		})
	}
}

func TestQuotaCacheReservationFailureAndRefund(t *testing.T) {
	for _, batch := range []bool{false, true} {
		for _, terminal := range []string{"token refusal", "refund", "zero", "consume"} {
			t.Run(fmt.Sprintf("batch=%t/%s", batch, terminal), func(t *testing.T) {
				db, c := quotaLifecycleFixture(t, batch, false, true)
				seedUserQuotaCache(t, "1000")
				if terminal == "token refusal" {
					require.NoError(t, db.Model(&model.Token{}).Where("id = ?", c.GetInt("token_id")).Update("remain_quota", 19).Error)
				}
				q := relay_util.NewQuota(c, "quota-fixture", 0)
				preErr := q.PreQuotaConsumption()
				if terminal == "token refusal" {
					require.NotNil(t, preErr)
				} else {
					require.Nil(t, preErr)
					balance, err := model.CacheGetUserQuota(1)
					require.NoError(t, err)
					require.Equal(t, 980, balance)
				}
				spent := 0
				switch terminal {
				case "zero", "consume":
					if terminal == "consume" {
						spent = 7
					}
					q.Consume(c, &types.Usage{PromptTokens: spent}, false)
				default:
					q.Undo(c)
				}
				model.FlushQuotaBatchForTest()
				balance, err := model.CacheGetUserQuota(1)
				require.NoError(t, err)
				require.Equal(t, 1000-spent, balance)
				// A subsequent valid request must not inherit a failed reservation.
				require.NoError(t, db.Model(&model.Token{}).Where("id = ?", c.GetInt("token_id")).Update("remain_quota", 1000).Error)
				next := relay_util.NewQuota(c, "quota-fixture", 980)
				if spent == 0 {
					require.Nil(t, next.PreQuotaConsumption())
					next.Undo(c)
				}
			})
		}
	}
}

func TestQuotaCacheUnavailableDoesNotInterruptAccounting(t *testing.T) {
	for _, batch := range []bool{false, true} {
		for _, outage := range []string{"before reserve", "after reserve"} {
			t.Run(fmt.Sprintf("batch=%t/%s", batch, outage), func(t *testing.T) {
				db, c := quotaLifecycleFixture(t, batch, false, true)
				if outage == "before reserve" {
					unavailableQuotaRedis(t)
				}
				q := relay_util.NewQuota(c, "quota-fixture", 0)
				require.Nil(t, q.PreQuotaConsumption())
				if outage == "after reserve" {
					unavailableQuotaRedis(t)
				}
				q.Consume(c, &types.Usage{PromptTokens: 7}, false)
				model.FlushQuotaBatchForTest()
				quotaBalances(t, db, c, 7, 1)
				var channel model.Channel
				require.NoError(t, db.First(&channel, 1).Error)
				require.EqualValues(t, 7, channel.UsedQuota)
			})
		}
	}
}

func TestQuotaCacheDatabaseErrorCannotUseStaleBalance(t *testing.T) {
	db, c := quotaLifecycleFixture(t, false, false, true)
	seedUserQuotaCache(t, "1000")
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	_, err = model.CacheGetUserQuota(1)
	require.Error(t, err)
	require.NotNil(t, relay_util.NewQuota(c, "quota-fixture", 0).PreQuotaConsumption())
}

func TestQuotaCacheMJRefundControl(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		t.Run(fmt.Sprint(unavailable), func(t *testing.T) {
			db, _ := quotaLifecycleFixture(t, false, false, true)
			require.NoError(t, db.AutoMigrate(&model.Midjourney{}))
			oldClient := requester.HTTPClient
			requester.HTTPClient = &http.Client{}
			t.Cleanup(func() { requester.HTTPClient = oldClient })
			task := &model.Midjourney{UserId: 1, ChannelId: 1, MjId: "cache-refund", Status: "IN_PROGRESS", Progress: "20%", SubmitTime: time.Now().UnixMilli(), Quota: 20}
			require.NoError(t, db.Create(task).Error)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode([]mjprovider.MidjourneyDto{{MjId: task.MjId, Status: "FAILURE", Progress: "100%", FailReason: "fixture failure", SubmitTime: task.SubmitTime}})
			}))
			defer server.Close()
			channel := &model.Channel{Id: 1, BaseURL: &server.URL, Key: "fixture-only"}
			if unavailable {
				unavailableQuotaRedis(t)
			}
			require.NoError(t, controller.MjTaskHandler(channel, []string{task.MjId}, map[string]*model.Midjourney{task.MjId: task}))
			balance, err := model.CacheGetUserQuota(1)
			require.NoError(t, err)
			require.Equal(t, 1020, balance)
		})
	}
}
