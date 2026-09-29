package model_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"one-api/common"
	"one-api/common/config"
	"one-api/common/logger"
	"one-api/model"
)

var quotaFixtureSequence atomic.Int64

// External engines are opt-in disposable loopback services, never application DSNs.
func quotaTransactionFixture(t *testing.T, batch, unlimited bool) (*gorm.DB, model.Token) {
	t.Helper()
	var dialect gorm.Dialector
	switch engine := os.Getenv("ONEHUB_QUOTA_TEST_DB"); engine {
	case "", "sqlite":
		dialect = sqlite.Open(filepath.Join(t.TempDir(), "quota.db") + "?_busy_timeout=10000&_journal_mode=WAL")
	case "mysql":
		dialect = mysql.Open("onehub_quota_test:quota-fixture-password@tcp(127.0.0.1:33306)/onehub_quota_test?charset=utf8mb4&parseTime=True&loc=UTC")
	case "postgres":
		dialect = postgres.Open("host=127.0.0.1 port=35432 user=onehub_quota_test password=quota-fixture-password dbname=onehub_quota_test sslmode=disable")
	default:
		t.Fatalf("unsupported isolated quota test engine %q", engine)
	}
	prefix := fmt.Sprintf("quota_fixture_%d_%d_", os.Getpid(), quotaFixtureSequence.Add(1))
	db, err := gorm.Open(dialect, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: prefix}})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	oldDB, oldLogger := model.DB, logger.Logger
	oldBatch, oldThreshold := config.BatchUpdateEnabled, config.QuotaRemindThreshold
	model.DB, logger.Logger = db, zap.NewNop()
	config.BatchUpdateEnabled, config.QuotaRemindThreshold = batch, 0
	t.Cleanup(func() {
		model.FlushQuotaBatchForTest()
		require.NoError(t, db.Migrator().DropTable(&model.Token{}, &model.User{}))
		require.NoError(t, sqlDB.Close())
		model.DB, logger.Logger = oldDB, oldLogger
		config.BatchUpdateEnabled, config.QuotaRemindThreshold = oldBatch, oldThreshold
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}))
	viper.Set("user_token_secret", "quota-transaction-fixture-only")
	require.NoError(t, common.InitUserToken())
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "quota-fixture", Quota: 1000}).Error)
	token := model.Token{UserId: 1, RemainQuota: 1000, UnlimitedQuota: unlimited}
	require.NoError(t, db.Create(&token).Error)
	return db, token
}

func requireQuotaPair(t *testing.T, db *gorm.DB, tokenID, userQuota, tokenQuota, used int) {
	t.Helper()
	var user model.User
	var token model.Token
	require.NoError(t, db.Unscoped().First(&user, 1).Error)
	require.NoError(t, db.Unscoped().First(&token, tokenID).Error)
	require.Equal(t, userQuota, user.Quota)
	require.Equal(t, tokenQuota, token.RemainQuota)
	require.Equal(t, used, token.UsedQuota)
}

func TestQuotaTransactionNormal(t *testing.T) {
	for _, batch := range []bool{false, true} {
		for _, unlimited := range []bool{false, true} {
			t.Run(fmt.Sprintf("batch=%t/unlimited=%t", batch, unlimited), func(t *testing.T) {
				db, token := quotaTransactionFixture(t, batch, unlimited)
				for _, operation := range []struct {
					pre           bool
					amount, spent int
				}{
					{true, 20, 20}, {false, -13, 7}, {false, 28, 35}, {false, -35, 0}, {true, 0, 0}, {false, 0, 0},
				} {
					if operation.pre {
						require.NoError(t, model.PreConsumeTokenQuota(token.Id, operation.amount))
					} else {
						require.NoError(t, model.PostConsumeTokenQuotaWithInfo(token.Id, 1, unlimited, operation.amount))
					}
					tokenSpent := operation.spent
					if unlimited {
						tokenSpent = 0
					}
					requireQuotaPair(t, db, token.Id, 1000-operation.spent, 1000-tokenSpent, tokenSpent)
					model.FlushQuotaBatchForTest()
					requireQuotaPair(t, db, token.Id, 1000-operation.spent, 1000-tokenSpent, tokenSpent)
				}
				require.Error(t, model.PreConsumeTokenQuota(token.Id, -1))
				require.Error(t, model.PreConsumeTokenQuota(token.Id, 1001))
				requireQuotaPair(t, db, token.Id, 1000, 1000, 0)
			})
		}
	}
}

func TestQuotaTransactionRollback(t *testing.T) {
	for _, operation := range []string{"pre", "settle", "refund"} {
		for _, table := range []string{"users", "tokens"} {
			t.Run(operation+"/"+table, func(t *testing.T) {
				db, token := quotaTransactionFixture(t, false, false)
				require.NoError(t, db.Callback().Update().Before("gorm:update").Register("quota_failure", func(tx *gorm.DB) {
					if tx.Statement.Schema != nil && strings.HasSuffix(tx.Statement.Schema.Table, "_"+table) {
						tx.AddError(errors.New("isolated quota write failure"))
					}
				}))
				var err error
				switch operation {
				case "pre":
					err = model.PreConsumeTokenQuota(token.Id, 20)
				case "settle":
					err = model.PostConsumeTokenQuotaWithInfo(token.Id, 1, false, 20)
				case "refund":
					err = model.PostConsumeTokenQuotaWithInfo(token.Id, 1, false, -20)
				}
				require.Error(t, err)
				requireQuotaPair(t, db, token.Id, 1000, 1000, 0)
			})
		}
	}
}

func TestQuotaTransactionInvalidRows(t *testing.T) {
	for _, unlimited := range []bool{false, true} {
		for _, change := range []string{"missing user", "deleted user", "missing token", "deleted token", "different owner"} {
			t.Run(fmt.Sprintf("unlimited=%t/%s", unlimited, change), func(t *testing.T) {
				db, token := quotaTransactionFixture(t, false, unlimited)
				switch change {
				case "missing user":
					require.NoError(t, db.Unscoped().Delete(&model.User{}, 1).Error)
				case "deleted user":
					require.NoError(t, db.Delete(&model.User{}, 1).Error)
				case "missing token":
					require.NoError(t, db.Unscoped().Delete(&token).Error)
				case "deleted token":
					require.NoError(t, db.Delete(&token).Error)
				case "different owner":
					require.NoError(t, db.Model(&token).Update("user_id", 2).Error)
				}
				for _, amount := range []int{20, -20} {
					require.Error(t, model.PostConsumeTokenQuotaWithInfo(token.Id, 1, unlimited, amount))
					var user model.User
					if err := db.Unscoped().First(&user, 1).Error; err == nil {
						require.Equal(t, 1000, user.Quota)
					}
					var current model.Token
					if err := db.Unscoped().First(&current, token.Id).Error; err == nil {
						require.Equal(t, 1000, current.RemainQuota)
						require.Zero(t, current.UsedQuota)
					}
				}
				require.Error(t, model.PreConsumeTokenQuota(token.Id, 20))
			})
		}
	}
}

func TestQuotaTransactionExhaustionAndActualUsage(t *testing.T) {
	for _, unlimited := range []bool{false, true} {
		t.Run(fmt.Sprint(unlimited), func(t *testing.T) {
			db, token := quotaTransactionFixture(t, true, unlimited)
			require.NoError(t, model.PreConsumeTokenQuota(token.Id, 1000))
			require.Error(t, model.PreConsumeTokenQuota(token.Id, 1))
			// Revocation stops future authorization but must not discard usage that
			// already occurred. Over-estimate settlement remains a signed delta.
			require.NoError(t, db.Model(&token).Updates(map[string]any{"status": config.TokenStatusDisabled, "expired_time": 1}).Error)
			require.NoError(t, model.PostConsumeTokenQuotaWithInfo(token.Id, 1, unlimited, 20))
			tokenQuota, used := -20, 1020
			if unlimited {
				tokenQuota, used = 1000, 0
			}
			requireQuotaPair(t, db, token.Id, -20, tokenQuota, used)
			require.NoError(t, model.PostConsumeTokenQuotaWithInfo(token.Id, 1, unlimited, -1020))
			requireQuotaPair(t, db, token.Id, 1000, 1000, 0)
		})
	}
}

func TestQuotaTransactionSnapshotChange(t *testing.T) {
	for _, unlimited := range []bool{false, true} {
		for _, change := range []string{"mode", "owner", "deleted"} {
			t.Run(fmt.Sprintf("unlimited=%t/%s", unlimited, change), func(t *testing.T) {
				db, token := quotaTransactionFixture(t, false, unlimited)
				require.NoError(t, db.Create(&model.User{Id: 2, Username: "other-fixture", AccessToken: "other-fixture-access", AffCode: "other-fixture-aff", Quota: 1000}).Error)
				var changed bool
				require.NoError(t, db.Callback().Update().Before("gorm:update").Register("quota_snapshot_change", func(tx *gorm.DB) {
					if changed || tx.Statement.Schema == nil || !strings.HasSuffix(tx.Statement.Schema.Table, "_users") {
						return
					}
					changed = true
					// Runs after the initial token read, before either balance write.
					switch change {
					case "mode":
						require.NoError(t, db.Model(&token).Update("unlimited_quota", !unlimited).Error)
					case "owner":
						require.NoError(t, db.Model(&token).Update("user_id", 2).Error)
					case "deleted":
						require.NoError(t, db.Delete(&token).Error)
					}
				}))
				require.Error(t, model.PreConsumeTokenQuota(token.Id, 20))
				require.True(t, changed)
				requireQuotaPair(t, db, token.Id, 1000, 1000, 0)
			})
		}
	}
}

func TestQuotaTransactionConcurrentReservations(t *testing.T) {
	for _, sharedUser := range []bool{false, true} {
		t.Run(fmt.Sprintf("sharedUser=%t", sharedUser), func(t *testing.T) {
			db, token := quotaTransactionFixture(t, true, false)
			const budget, amount, workers = 95, 10, 24
			if sharedUser {
				require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("quota", budget).Error)
			} else {
				require.NoError(t, db.Model(&token).Update("remain_quota", budget).Error)
			}
			ids := make([]int, workers)
			for i := range ids {
				ids[i] = token.Id
				if sharedUser {
					other := model.Token{UserId: 1, RemainQuota: 1000}
					require.NoError(t, db.Create(&other).Error)
					ids[i] = other.Id
				}
			}
			var accepted atomic.Int64
			var wg sync.WaitGroup
			start := make(chan struct{})
			for _, id := range ids {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					<-start
					if model.PreConsumeTokenQuota(id, amount) == nil {
						accepted.Add(1)
					}
				}(id)
			}
			close(start)
			wg.Wait()
			require.EqualValues(t, budget/amount, accepted.Load())
			spent := int(accepted.Load()) * amount
			for _, flush := range []bool{false, true} {
				if flush {
					model.FlushQuotaBatchForTest()
				}
				var user model.User
				require.NoError(t, db.First(&user, 1).Error)
				initial := 1000
				if sharedUser {
					initial = budget
				}
				require.Equal(t, initial-spent, user.Quota)
				var tokens []model.Token
				require.NoError(t, db.Find(&tokens).Error)
				total := 0
				for _, current := range tokens {
					require.GreaterOrEqual(t, current.RemainQuota, 0)
					total += current.UsedQuota
				}
				require.Equal(t, spent, total)
			}
		})
	}
}
