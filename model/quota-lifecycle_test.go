package model_test

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"one-api/common/config"
	"one-api/model"
	"one-api/relay/relay_util"
	"one-api/types"
)

func quotaLifecycleFixture(t *testing.T, batch, unlimited bool, redisMode ...bool) (*gorm.DB, *gin.Context) {
	t.Helper()
	redisEnabled := len(redisMode) > 0 && redisMode[0]
	db, token := tokenAuthorityFixture(t, redisEnabled, batch, false, unlimited)
	oldPricing, oldPre, oldLogs := model.PricingInstance, config.PreConsumedQuota, config.LogConsumeEnabled
	oldThreshold := config.QuotaRemindThreshold
	model.PricingInstance = &model.Pricing{Prices: map[string]*model.Price{
		"quota-fixture": {Type: model.TokensPriceType, Input: 1, Output: 1},
		"quota-free":    {Type: model.TokensPriceType},
	}}
	config.PreConsumedQuota, config.LogConsumeEnabled, config.QuotaRemindThreshold = 20, true, 0
	t.Cleanup(func() {
		model.PricingInstance, config.PreConsumedQuota, config.LogConsumeEnabled = oldPricing, oldPre, oldLogs
		config.QuotaRemindThreshold = oldThreshold
	})
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Log{}, &model.Task{}, &model.QuotaReservation{}))
	require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("quota", 1000).Error)
	require.NoError(t, db.Model(&token).Update("remain_quota", 1000).Error)
	require.NoError(t, db.Create(&model.Channel{Id: 1}).Error)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/fixture", nil)
	c.Set("id", 1)
	c.Set("token_id", token.Id)
	c.Set("token_unlimited_quota", unlimited)
	c.Set("channel_id", 1)
	c.Set("group_ratio", float64(1))
	c.Set("requestStartTime", time.Now())
	return db, c
}

func quotaBalances(t *testing.T, db *gorm.DB, c *gin.Context, spent, requests int) {
	t.Helper()
	var user model.User
	var token model.Token
	require.NoError(t, db.First(&user, 1).Error)
	require.NoError(t, db.First(&token, c.GetInt("token_id")).Error)
	require.Equal(t, 1000-spent, user.Quota)
	require.Equal(t, requests, user.RequestCount)
	if c.GetBool("token_unlimited_quota") {
		require.Equal(t, 1000, token.RemainQuota)
		require.Zero(t, token.UsedQuota)
	} else {
		require.Equal(t, 1000-spent, token.RemainQuota)
		require.Equal(t, spent, token.UsedQuota)
	}
	var logs int64
	require.NoError(t, db.Model(&model.Log{}).Count(&logs).Error)
	require.EqualValues(t, requests, logs)
}

func TestQuotaLifecycleTerminalOnce(t *testing.T) {
	for _, redisEnabled := range []bool{false, true} {
		for _, batch := range []bool{false, true} {
			for _, unlimited := range []bool{false, true} {
				for _, operation := range []string{"refund", "consume", "zero", "refund then consume", "consume then refund"} {
					t.Run(fmt.Sprintf("redis=%t/batch=%t/unlimited=%t/%s", redisEnabled, batch, unlimited, operation), func(t *testing.T) {
						db, c := quotaLifecycleFixture(t, batch, unlimited, redisEnabled)
						q := relay_util.NewQuota(c, "quota-fixture", 0)
						require.Nil(t, q.PreQuotaConsumption())
						spent, requests := 0, 0
						switch operation {
						case "refund", "refund then consume":
							q.Undo(c)
							q.Undo(c)
							if operation == "refund then consume" {
								q.Consume(c, &types.Usage{PromptTokens: 7}, false)
							}
						default:
							spent, requests = 7, 1
							if operation == "zero" {
								spent = 0
							}
							q.Consume(c, &types.Usage{PromptTokens: spent}, false)
							q.Consume(c, &types.Usage{PromptTokens: spent}, false)
							if operation == "consume then refund" {
								q.Undo(c)
							}
						}
						model.FlushQuotaBatchForTest()
						quotaBalances(t, db, c, spent, requests)
					})
				}
			}
		}
	}

}

func TestQuotaLifecycleConcurrentTerminal(t *testing.T) {
	db, c := quotaLifecycleFixture(t, false, false)
	q := relay_util.NewQuota(c, "quota-fixture", 0)
	require.Nil(t, q.PreQuotaConsumption())
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				q.Undo(c)
			} else {
				q.Consume(c, &types.Usage{PromptTokens: 7}, false)
			}
		}(i)
	}
	wg.Wait()
	var user model.User
	require.NoError(t, db.First(&user, 1).Error)
	require.Contains(t, []int{1000, 993}, user.Quota)
	if user.Quota == 1000 {
		quotaBalances(t, db, c, 0, 0)
	} else {
		quotaBalances(t, db, c, 7, 1)
	}
}

func TestQuotaLifecycleNormalSettlement(t *testing.T) {
	for _, amount := range []int{0, 7, 20, 35} {
		t.Run(fmt.Sprint(amount), func(t *testing.T) {
			db, c := quotaLifecycleFixture(t, false, false)
			q := relay_util.NewQuota(c, "quota-fixture", 0)
			require.Nil(t, q.PreQuotaConsumption())
			q.Consume(c, &types.Usage{PromptTokens: amount}, false)
			quotaBalances(t, db, c, amount, 1)
		})
	}
	for _, mode := range []string{"high balance", "free", "without preconsume"} {
		t.Run(mode, func(t *testing.T) {
			db, c := quotaLifecycleFixture(t, false, false)
			name := "quota-fixture"
			if mode == "free" {
				name = "quota-free"
			}
			if mode == "high balance" {
				require.NoError(t, db.Model(&model.User{}).Where("id=1").Update("quota", 10000).Error)
			}
			q := relay_util.NewQuota(c, name, 0)
			if mode != "without preconsume" {
				require.Nil(t, q.PreQuotaConsumption())
			}
			q.Consume(c, &types.Usage{PromptTokens: 7}, false)
			if mode == "high balance" {
				require.NoError(t, db.Model(&model.User{}).Where("id=1").Update("quota", gorm.Expr("quota - 9000")).Error)
			}
			spent := 7
			if mode == "free" {
				spent = 0
			}
			quotaBalances(t, db, c, spent, 1)
		})
	}
}

func TestQuotaLifecycleWriteFailureKeepsPendingIntent(t *testing.T) {
	for _, refund := range []bool{false, true} {
		t.Run(fmt.Sprint(refund), func(t *testing.T) {
			db, c := quotaLifecycleFixture(t, false, false)
			q := relay_util.NewQuota(c, "quota-fixture", 0)
			require.Nil(t, q.PreQuotaConsumption())
			var attempts atomic.Int32
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("fixture_token_failure", func(tx *gorm.DB) {
				if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "tokens" {
					attempts.Add(1)
					tx.AddError(errors.New("fixture token write failure"))
				}
			}))
			if refund {
				q.Undo(c)
			} else {
				q.Consume(c, &types.Usage{PromptTokens: 7}, false)
			}
			q.Undo(c)
			q.Consume(c, &types.Usage{PromptTokens: 7}, false)
			require.EqualValues(t, 3, attempts.Load(), "retries must attempt the persisted intent without partial charges")
			var user model.User
			var token model.Token
			require.NoError(t, db.First(&user, 1).Error)
			require.NoError(t, db.First(&token, c.GetInt("token_id")).Error)
			require.Equal(t, 980, user.Quota)
			require.Equal(t, 980, token.RemainQuota, "failed settlement keeps the complete reservation")
		})
	}
}

func TestQuotaLifecycleRejectsNegativeSettlement(t *testing.T) {
	db, c := quotaLifecycleFixture(t, false, false)
	model.PricingInstance.Prices["quota-fixture"].Input = 0
	q := relay_util.NewQuota(c, "quota-fixture", 0)
	require.Nil(t, q.PreQuotaConsumption())
	q.Consume(c, &types.Usage{CompletionTokens: -1}, false)
	q.Undo(c)
	quotaBalances(t, db, c, 20, 0)
}
