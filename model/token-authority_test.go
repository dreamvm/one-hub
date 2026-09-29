package model_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	redisclient "github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"one-api/common"
	"one-api/common/cache"
	"one-api/common/config"
	"one-api/common/logger"
	"one-api/common/redis"
	"one-api/model"
)

// InitUserToken is a startup operation: its HMAC pool retains the initialized
// key. All fixtures in this test binary must use the same synthetic secret.
const quotaFixtureTokenSecret = "quota-shared-fixture-only"

// Exercise the production Redis store and MessagePack marshaler without network
// access. Only the Redis commands needed by token authentication are simulated.
type tokenCacheFixture struct {
	sync.Mutex
	values map[string]string
}

func (f *tokenCacheFixture) DialHook(_ redisclient.DialHook) redisclient.DialHook {
	return func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("token fixture forbids network")
	}
}

func (f *tokenCacheFixture) ProcessHook(_ redisclient.ProcessHook) redisclient.ProcessHook {
	return func(_ context.Context, cmd redisclient.Cmder) error {
		f.Lock()
		defer f.Unlock()
		key, _ := cmd.Args()[1].(string)
		switch cmd.Name() {
		case "get":
			value, exists := f.values[key]
			if !exists {
				cmd.SetErr(redisclient.Nil)
				return redisclient.Nil
			}
			cmd.(*redisclient.StringCmd).SetVal(value)
		case "set":
			switch value := cmd.Args()[2].(type) {
			case []byte:
				f.values[key] = string(value)
			case string:
				f.values[key] = value
			default:
				return fmt.Errorf("unexpected fixture value type %T", value)
			}
			cmd.(*redisclient.StatusCmd).SetVal("OK")
		case "decrby":
			current, err := strconv.ParseInt(f.values[key], 10, 64)
			if err != nil {
				return err
			}
			delta, err := strconv.ParseInt(fmt.Sprint(cmd.Args()[2]), 10, 64)
			if err != nil {
				return err
			}
			f.values[key] = strconv.FormatInt(current-delta, 10)
			cmd.(*redisclient.IntCmd).SetVal(current - delta)
		case "sismember":
			cmd.(*redisclient.BoolCmd).SetVal(key == model.OldUserTokensCacheKey)
		default:
			return fmt.Errorf("unexpected fixture command %s", cmd.Name())
		}
		return nil
	}
}

func (f *tokenCacheFixture) ProcessPipelineHook(_ redisclient.ProcessPipelineHook) redisclient.ProcessPipelineHook {
	return func(context.Context, []redisclient.Cmder) error { return errors.New("unexpected fixture pipeline") }
}

func tokenAuthorityFixture(t *testing.T, redisEnabled, batchEnabled, legacy, unlimited bool) (*gorm.DB, model.Token) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "tokens.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	oldDB, oldLogger, oldRedis := model.DB, logger.Logger, redis.RDB
	oldRedisEnabled, oldBatch, oldCacheSeconds := config.RedisEnabled, config.BatchUpdateEnabled, model.TokenCacheSeconds
	model.DB, logger.Logger = db, zap.NewNop()
	config.RedisEnabled, config.BatchUpdateEnabled, model.TokenCacheSeconds = redisEnabled, batchEnabled, 3600
	client := redisclient.NewClient(&redisclient.Options{Addr: "fixture.invalid:6379"})
	client.AddHook(&tokenCacheFixture{values: make(map[string]string)})
	redis.RDB = client
	cache.InitCacheManager()
	t.Cleanup(func() {
		model.DB, logger.Logger, redis.RDB = oldDB, oldLogger, oldRedis
		config.RedisEnabled, config.BatchUpdateEnabled, model.TokenCacheSeconds = oldRedisEnabled, oldBatch, oldCacheSeconds
		cache.InitCacheManager()
		_ = client.Close()
		_ = sqlDB.Close()
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}))
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "token-authority-fixture", Status: config.UserStatusEnabled, Quota: 1000000}).Error)
	viper.Set("user_token_secret", quotaFixtureTokenSecret)
	require.NoError(t, common.InitUserToken())
	token := model.Token{UserId: 1, Name: "current token", Status: config.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 100, UnlimitedQuota: unlimited}
	require.NoError(t, db.Create(&token).Error)
	if legacy {
		token.Key = strings.Repeat("l", 48)
		require.NoError(t, db.Model(&token).Update("key", token.Key).Error)
	}
	require.NoError(t, db.First(&token, token.Id).Error)
	_, err = model.ValidateUserToken(token.Key)
	require.NoError(t, err, "legitimate current token must authenticate")
	// Persist a legacy cache object explicitly; it must remain harmless even if
	// invalidation was missed or an older cache fill completed after a DB write.
	require.NoError(t, cache.SetCache(fmt.Sprintf(model.UserTokensKey, token.Key), &token, time.Hour))
	return db, token
}

func TestTokenAuthorityRejectsStaleAuthorization(t *testing.T) {
	for _, redisEnabled := range []bool{false, true} {
		for _, batchEnabled := range []bool{false, true} {
			for _, legacy := range []bool{false, true} {
				for _, change := range []string{"exhausted", "limited", "disabled", "expired", "deleted", "rotated", "database unavailable"} {
					name := fmt.Sprintf("redis=%t/batch=%t/legacy=%t/%s", redisEnabled, batchEnabled, legacy, change)
					t.Run(name, func(t *testing.T) {
						db, token := tokenAuthorityFixture(t, redisEnabled, batchEnabled, legacy, change == "limited")
						authKey := token.Key
						switch change {
						case "exhausted", "limited":
							require.NoError(t, db.Model(&token).Updates(map[string]any{"remain_quota": 0, "unlimited_quota": false}).Error)
						case "disabled":
							require.NoError(t, db.Model(&token).Update("status", config.TokenStatusDisabled).Error)
						case "expired":
							require.NoError(t, db.Model(&token).Update("expired_time", 1).Error)
						case "deleted":
							require.NoError(t, db.Delete(&token).Error)
						case "rotated":
							require.NoError(t, db.Model(&token).Update("key", "rotated-fixture-key").Error)
						case "database unavailable":
							sqlDB, err := db.DB()
							require.NoError(t, err)
							require.NoError(t, sqlDB.Close())
						}
						accepted, err := model.ValidateUserToken(authKey)
						require.Error(t, err, "cached token must not override persisted authorization")
						require.Nil(t, accepted)
					})
				}
			}
		}
	}
}

func TestTokenAuthorityReturnsCurrentFieldsAndRestoredQuota(t *testing.T) {
	for _, unlimited := range []bool{false, true} {
		t.Run(fmt.Sprintf("unlimited=%t", unlimited), func(t *testing.T) {
			db, token := tokenAuthorityFixture(t, true, true, false, unlimited)
			stale := token
			stale.RemainQuota = 0
			stale.UnlimitedQuota = false
			require.NoError(t, cache.SetCache(fmt.Sprintf(model.UserTokensKey, token.Key), &stale, time.Hour))
			remaining := 5
			if unlimited {
				remaining = 0
			}
			require.NoError(t, db.Model(&token).Updates(map[string]any{"name": "updated", "group": "current", "remain_quota": remaining}).Error)
			current, err := model.ValidateUserToken(token.Key)
			require.NoError(t, err)
			require.Equal(t, "updated", current.Name)
			require.Equal(t, "current", current.Group)
			require.Equal(t, remaining, current.RemainQuota)
			require.Equal(t, unlimited, current.UnlimitedQuota)
		})
	}
}
