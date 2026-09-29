package model_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"one-api/controller"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"one-api/common"
	"one-api/common/config"
	"one-api/model"
)

func redemptionFixture(t *testing.T, batch bool) (*gorm.DB, model.Redemption) {
	t.Helper()
	db, _ := quotaLedgerFixture(t, batch, false)
	oldPG, oldSQLite, oldRedis := common.UsingPostgreSQL, common.UsingSQLite, config.RedisEnabled
	common.UsingPostgreSQL = db.Dialector.Name() == "postgres"
	common.UsingSQLite = db.Dialector.Name() == "sqlite"
	config.RedisEnabled = false
	t.Cleanup(func() { common.UsingPostgreSQL, common.UsingSQLite, config.RedisEnabled = oldPG, oldSQLite, oldRedis })
	for _, table := range []any{&model.Redemption{}, &model.UserGroup{}} {
		require.NoError(t, db.AutoMigrate(table))
	}
	t.Cleanup(func() { require.NoError(t, db.Migrator().DropTable(&model.Redemption{}, &model.UserGroup{})) })
	code := model.Redemption{UserId: 99, Key: "redemption-fixture-only", Name: "fixture", Quota: 100, Status: config.RedemptionCodeStatusEnabled}
	require.NoError(t, code.Insert())
	return db, code
}

func TestQuotaTransactionRedemptionControlAndStaleEdit(t *testing.T) {
	for _, staleEdit := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "stale admin edit"}[staleEdit], func(t *testing.T) {
			db, code := redemptionFixture(t, false)
			quota, err := model.Redeem(code.Key, 1, "127.0.0.1")
			require.NoError(t, err)
			require.Equal(t, 100, quota)
			if staleEdit {
				code.Name = "renamed"
				require.Error(t, code.Update(), "stale editor must not reopen a used code")
			}
			_, err = model.Redeem(code.Key, 1, "127.0.0.1")
			require.Error(t, err)
			var user model.User
			require.NoError(t, db.First(&user, 1).Error)
			require.Equal(t, 1100, user.Quota)
		})
	}
}

func TestQuotaTransactionRedemptionFailureIsAtomic(t *testing.T) {
	for _, stage := range []string{"missing user", "log write"} {
		t.Run(stage, func(t *testing.T) {
			db, code := redemptionFixture(t, true)
			userID := 1
			if stage == "missing user" {
				userID = 2
			} else {
				require.NoError(t, db.Callback().Create().Before("gorm:create").Register("redemption_log_fault", func(tx *gorm.DB) {
					if tx.Statement.Schema != nil && strings.HasSuffix(tx.Statement.Schema.Table, "_logs") {
						tx.AddError(errors.New("fixture log unavailable"))
					}
				}))
				t.Cleanup(func() { require.NoError(t, db.Callback().Create().Remove("redemption_log_fault")) })
			}
			_, err := model.Redeem(code.Key, userID, "127.0.0.1")
			require.Error(t, err)
			var saved model.Redemption
			require.NoError(t, db.First(&saved, code.Id).Error)
			require.Equal(t, config.RedemptionCodeStatusEnabled, saved.Status)
			var user model.User
			require.NoError(t, db.First(&user, 1).Error)
			require.Equal(t, 1000, user.Quota)
		})
	}
}

func TestQuotaTransactionRedemptionConcurrent(t *testing.T) {
	for _, sameUser := range []bool{false, true} {
		t.Run(fmt.Sprint(sameUser), func(t *testing.T) {
			db, code := redemptionFixture(t, true)
			require.NoError(t, db.Create(&model.User{Id: 2, Username: "second-fixture", AccessToken: "redemption-second-fixture", AffCode: "second-fixture", Quota: 1000}).Error)
			// Force both legacy read-before-write transactions to see enabled. The
			// corrected path claims with a write first and never reads enabled here.
			var reads atomic.Int32
			ready := make(chan struct{})
			require.NoError(t, db.Callback().Query().After("gorm:query").Register("redemption_read_barrier", func(tx *gorm.DB) {
				row, ok := tx.Statement.Dest.(*model.Redemption)
				if !ok || row.Status != config.RedemptionCodeStatusEnabled {
					return
				}
				if reads.Add(1) == 2 {
					close(ready)
				}
				select {
				case <-ready:
				case <-time.After(5 * time.Second):
					tx.AddError(errors.New("fixture barrier timed out"))
				}
			}))
			var wg sync.WaitGroup
			results := make(chan error, 2)
			for id := 1; id <= 2; id++ {
				userID := id
				if sameUser {
					userID = 1
				}
				wg.Add(1)
				go func() { defer wg.Done(); _, err := model.Redeem(code.Key, userID, "telegram"); results <- err }()
			}
			wg.Wait()
			close(results)
			success := 0
			for err := range results {
				if err == nil {
					success++
				}
			}
			require.NoError(t, db.Callback().Query().Remove("redemption_read_barrier"))
			require.Equal(t, 1, success, "one code credits exactly one successful redeemer")
			model.FlushQuotaBatchForTest()
			var users []model.User
			require.NoError(t, db.Order("id").Find(&users).Error)
			require.Len(t, users, 2)
			require.Equal(t, 2100, users[0].Quota+users[1].Quota)
			var logs int64
			require.NoError(t, db.Model(&model.Log{}).Where("type = ?", model.LogTypeTopup).Count(&logs).Error)
			require.EqualValues(t, 1, logs)
		})
	}
}

func TestQuotaTransactionRedemptionControls(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(fmt.Sprint(batch), func(t *testing.T) {
			db, code := redemptionFixture(t, batch)
			code.Status = config.RedemptionCodeStatusDisabled
			require.NoError(t, code.SelectUpdate())
			_, err := model.Redeem(code.Key, 1, "telegram")
			require.Error(t, err)
			code.Status = config.RedemptionCodeStatusEnabled
			require.NoError(t, code.SelectUpdate())
			require.NoError(t, code.SelectUpdate(), "unchanged status remains a valid operation on MySQL")
			code.Name = "new name"
			code.Quota = 120
			require.NoError(t, code.Update())
			quota, err := model.Redeem(code.Key, 1, "telegram")
			require.NoError(t, err)
			require.Equal(t, 120, quota)
			model.FlushQuotaBatchForTest()
			requireQuotaPair(t, db, 1, 1120, 1000, 0)
			var saved model.Redemption
			require.NoError(t, db.First(&saved, code.Id).Error)
			require.Equal(t, 99, saved.UserId, "creator is not overwritten by redeemer")
			var redeemer int
			require.NoError(t, db.Model(&saved).Select("redeemed_by").Scan(&redeemer).Error)
			require.Equal(t, 1, redeemer)
			code.Status = config.RedemptionCodeStatusDisabled
			require.Error(t, code.SelectUpdate())
		})
	}
}

func TestQuotaTransactionRedemptionInvalidAndRollback(t *testing.T) {
	for _, stage := range []string{"negative", "zero", "overflow", "deleted user", "claim", "balance", "promotion", "log"} {
		t.Run(stage, func(t *testing.T) {
			db, code := redemptionFixture(t, false)
			switch stage {
			case "negative":
				require.NoError(t, db.Model(&code).Update("quota", -1).Error)
			case "zero":
				require.NoError(t, db.Model(&code).Update("quota", 0).Error)
			case "overflow":
				require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("quota", int(^uint(0)>>1)-10).Error)
			case "deleted user":
				require.NoError(t, db.Delete(&model.User{}, 1).Error)
			}
			inject := func(tx *gorm.DB) {
				if tx.Statement.Schema == nil {
					return
				}
				table := tx.Statement.Schema.Table
				if stage == "claim" && strings.HasSuffix(table, "_redemptions") || stage == "balance" && strings.HasSuffix(table, "_users") || stage == "promotion" && strings.HasSuffix(table, "_user_groups") || stage == "log" && strings.HasSuffix(table, "_logs") {
					tx.AddError(errors.New("redemption write failure"))
				}
			}
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("redemption_fault", inject))
			require.NoError(t, db.Callback().Create().Before("gorm:create").Register("redemption_fault", inject))
			require.NoError(t, db.Callback().Query().Before("gorm:query").Register("redemption_fault", inject))
			_, err := model.Redeem(code.Key, 1, "127.0.0.1")
			require.Error(t, err)
			require.NoError(t, db.Callback().Update().Remove("redemption_fault"))
			require.NoError(t, db.Callback().Create().Remove("redemption_fault"))
			require.NoError(t, db.Callback().Query().Remove("redemption_fault"))
			var saved model.Redemption
			require.NoError(t, db.First(&saved, code.Id).Error)
			require.Equal(t, config.RedemptionCodeStatusEnabled, saved.Status)
			require.Zero(t, saved.RedeemedTime)
			var user model.User
			require.NoError(t, db.Unscoped().First(&user, 1).Error)
			expected := 1000
			if stage == "overflow" {
				expected = int(^uint(0)>>1) - 10
			}
			require.Equal(t, expected, user.Quota)
			if stage == "claim" || stage == "balance" || stage == "promotion" || stage == "log" {
				_, err = model.Redeem(code.Key, 1, "127.0.0.1")
				require.NoError(t, err)
			}
		})
	}
}

func TestQuotaTransactionRedemptionPromotionCountsOnce(t *testing.T) {
	db, code := redemptionFixture(t, false)
	oldUnit := config.QuotaPerUnit
	config.QuotaPerUnit = 1
	t.Cleanup(func() { config.QuotaPerUnit = oldUnit })
	require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("group", "default").Error)
	require.NoError(t, db.Create(&model.UserGroup{Symbol: "too-early", Name: "fixture", Promotion: true, Min: 1150}).Error)
	require.NoError(t, db.Create(&model.UserGroup{Symbol: "earned", Name: "fixture", Promotion: true, Min: 1100, Max: 1150}).Error)
	_, err := model.Redeem(code.Key, 1, "127.0.0.1")
	require.NoError(t, err)
	var user model.User
	require.NoError(t, db.First(&user, 1).Error)
	require.Equal(t, "earned", user.Group)
}

func TestQuotaTransactionRedemptionHistoricalUsedMarkers(t *testing.T) {
	for _, field := range []string{"redeemed_time", "redeemed_by"} {
		t.Run(field, func(t *testing.T) {
			db, code := redemptionFixture(t, false)
			require.NoError(t, db.Model(&code).Update(field, 1).Error)
			_, err := model.Redeem(code.Key, 1, "127.0.0.1")
			require.Error(t, err, "used evidence must override a stale enabled flag")
			requireQuotaPair(t, db, 1, 1000, 1000, 0)
			var count int64
			require.NoError(t, db.Model(&model.Log{}).Count(&count).Error)
			require.Zero(t, count)
		})
	}
}

func TestQuotaTransactionRedemptionNullUnusedMarkers(t *testing.T) {
	db, code := redemptionFixture(t, false)
	require.NoError(t, db.Model(&code).Updates(map[string]any{"redeemed_time": nil, "redeemed_by": nil}).Error)
	_, err := model.Redeem(code.Key, 1, "127.0.0.1")
	require.NoError(t, err)
	requireQuotaPair(t, db, 1, 1100, 1000, 0)
}

func TestRedemptionHTTPAndRedisControls(t *testing.T) {
	for _, outage := range []bool{false, true} {
		t.Run(fmt.Sprint(outage), func(t *testing.T) {
			db, c := quotaLifecycleFixture(t, true, false, true)
			for _, table := range []any{&model.Redemption{}, &model.UserGroup{}} {
				require.NoError(t, db.AutoMigrate(table))
			}
			require.NoError(t, db.Create(&model.UserGroup{Symbol: "recharged", Name: "fixture", Promotion: true}).Error)
			code := model.Redemption{UserId: 99, Key: "http-redemption-fixture", Quota: 100, Name: "fixture"}
			require.NoError(t, code.Insert())
			if outage {
				unavailableQuotaRedis(t)
			}
			router := gin.New()
			router.POST("/topup", func(ctx *gin.Context) { ctx.Set("id", c.GetInt("id")); controller.TopUp(ctx) })
			router.PUT("/redemption", controller.UpdateRedemption)
			response := httptest.NewRecorder()
			request := httptest.NewRequest("POST", "/topup", strings.NewReader(`{"key":"http-redemption-fixture"}`))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(response, request)
			require.JSONEq(t, `{"success":true,"message":"","data":100}`, response.Body.String())
			requireQuotaPair(t, db, c.GetInt("token_id"), 1100, 1000, 0)
			for _, query := range []string{"", "?status_only=true"} {
				response = httptest.NewRecorder()
				request = httptest.NewRequest("PUT", "/redemption"+query, strings.NewReader(fmt.Sprintf(`{"id":%d,"status":1,"name":"stale","quota":100}`, code.Id)))
				request.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(response, request)
				var body struct {
					Success bool `json:"success"`
				}
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
				require.False(t, body.Success)
			}
			var saved model.Redemption
			require.NoError(t, db.First(&saved, code.Id).Error)
			require.Equal(t, config.RedemptionCodeStatusUsed, saved.Status)
		})
	}
}
