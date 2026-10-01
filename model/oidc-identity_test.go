package model_test

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"one-api/common/config"
	"one-api/model"
)

// The existing three-database CI entry point includes these identity writes.
func TestQuotaTransactionOIDCIdentity(t *testing.T) {
	for _, stage := range []string{"concurrent registration", "exact identity", "legacy migration", "stale profile", "missing schema", "missing index", "migration failure", "duplicate key", "generic insertion"} {
		t.Run(stage, func(t *testing.T) {
			db, _ := quotaTransactionFixture(t, false, false)
			oldQuota, oldRedis := config.QuotaForNewUser, config.RedisEnabled
			config.QuotaForNewUser, config.RedisEnabled = 0, false
			t.Cleanup(func() { config.QuotaForNewUser, config.RedisEnabled = oldQuota, oldRedis })
			const issuer = "https://identity.fixture.invalid"
			const subject = "fixture-subject"
			switch stage {
			case "concurrent registration":
				config.QuotaForNewUser = 50
				require.NoError(t, db.AutoMigrate(&model.Log{}))
				t.Cleanup(func() { require.NoError(t, db.Migrator().DropTable(&model.Log{}, &model.Channel{})) })
				const workers = 8
				users, errs := make([]model.User, workers), make([]error, workers)
				start := make(chan struct{})
				var wg sync.WaitGroup
				for i := range users {
					wg.Add(1)
					go func(i int) {
						defer wg.Done()
						users[i] = model.User{Username: fmt.Sprintf("concurrent%d", i), Status: config.UserStatusEnabled}
						<-start
						errs[i] = users[i].InsertOIDC(issuer, subject)
					}(i)
				}
				close(start)
				wg.Wait()
				for i := range users {
					require.NoError(t, errs[i])
					require.Equal(t, users[0].Id, users[i].Id)
				}
				var count, total int64
				require.NoError(t, db.Model(&model.User{}).Where("oidc_id = ?", subject).Count(&count).Error)
				require.EqualValues(t, 1, count)
				require.NoError(t, db.Model(&model.User{}).Count(&total).Error)
				require.EqualValues(t, 2, total, "no orphan registration rows")
				require.NoError(t, db.Model(&model.Log{}).Where("user_id = ?", users[0].Id).Count(&count).Error)
				require.EqualValues(t, 1, count, "only the winning INSERT grants initial quota and records the gift")
				require.Equal(t, 50, users[0].Quota)
			case "exact identity":
				pairs := [][2]string{{issuer, subject}, {issuer, "FIXTURE-SUBJECT"}, {issuer, subject + " "}, {issuer + "/", subject}, {"https://other.fixture.invalid", subject}}
				ids := map[int]bool{}
				for i, pair := range pairs {
					u := model.User{Username: fmt.Sprintf("identity%d", i)}
					require.NoError(t, u.InsertOIDC(pair[0], pair[1]))
					require.False(t, ids[u.Id])
					ids[u.Id] = true
					var saved model.User
					require.NoError(t, saved.FillUserByOIDCIdentity(pair[0], pair[1]))
					require.Equal(t, u.Id, saved.Id)
				}
			case "legacy migration":
				require.NoError(t, db.Migrator().DropIndex(&model.User{}, "OidcIdentityKey"))
				require.NoError(t, db.Migrator().DropColumn(&model.User{}, "OidcIdentityKey"))
				require.NoError(t, db.Migrator().DropColumn(&model.User{}, "OidcIssuer"))
				require.NoError(t, db.Model(&model.User{}).Where("id = ?", 1).Update("oidc_id", subject).Error)
				for i := 0; i < 2; i++ {
					require.NoError(t, db.AutoMigrate(&model.User{}))
					var old model.User
					require.NoError(t, db.First(&old, 1).Error)
					require.Equal(t, subject, old.OidcId)
					require.Empty(t, old.OidcIssuer)
					require.Nil(t, old.OidcIdentityKey)
					var lookup model.User
					require.ErrorIs(t, lookup.FillUserByOIDCIdentity(issuer, subject), model.ErrOIDCIdentityUnresolved)
				}
			case "stale profile":
				u := model.User{Username: "before-unbind"}
				require.NoError(t, u.InsertOIDC(issuer, subject))
				require.NoError(t, db.Model(&model.User{}).Where("id = ?", u.Id).Updates(map[string]any{"oidc_id": "", "oidc_issuer": "", "oidc_identity_key": nil}).Error)
				u.DisplayName = "After unbind"
				require.NoError(t, u.Update(false))
				var saved model.User
				require.NoError(t, db.First(&saved, u.Id).Error)
				require.Empty(t, saved.OidcId)
				require.Empty(t, saved.OidcIssuer)
				require.Nil(t, saved.OidcIdentityKey)
				replacement := model.User{Username: "after-unbind"}
				require.NoError(t, replacement.InsertOIDC(issuer, subject))
				require.NotEqual(t, u.Id, replacement.Id)
			case "missing schema":
				require.NoError(t, db.Migrator().DropIndex(&model.User{}, "OidcIdentityKey"))
				require.NoError(t, db.Migrator().DropColumn(&model.User{}, "OidcIdentityKey"))
				u := model.User{Username: "no-fallback"}
				require.Error(t, u.InsertOIDC(issuer, subject))
			case "missing index", "migration failure":
				u := model.User{Username: "before-index-loss"}
				require.NoError(t, u.InsertOIDC(issuer, subject))
				require.NoError(t, db.Migrator().DropIndex(&model.User{}, "OidcIdentityKey"))
				if stage == "migration failure" {
					duplicate := model.User{Username: "inconsistent", AccessToken: "different", AffCode: "different", OidcId: subject, OidcIssuer: issuer, OidcIdentityKey: u.OidcIdentityKey}
					require.NoError(t, db.Create(&duplicate).Error)
					require.Error(t, db.AutoMigrate(&model.User{}), "duplicate identity data must fail migration, never choose an owner")
				}
				var lookup model.User
				require.Error(t, lookup.FillUserByOIDCIdentity(issuer, subject))
				newUser := model.User{Username: "no-index-fallback"}
				require.Error(t, newUser.InsertOIDC(issuer, "new-subject"))
			case "duplicate key":
				u := model.User{Username: "first-owner"}
				require.NoError(t, u.InsertOIDC(issuer, subject))
				duplicate := model.User{Username: "second-owner", AccessToken: "different", AffCode: "different", OidcId: subject, OidcIssuer: issuer, OidcIdentityKey: u.OidcIdentityKey}
				require.Error(t, db.Create(&duplicate).Error, "database unique constraint protects independent processes")
				require.NoError(t, db.Delete(&u).Error)
				require.Error(t, db.Create(&duplicate).Error, "soft-deleted ownership stays reserved")
			case "generic insertion":
				u := model.User{Username: "unknown-owner", OidcId: subject}
				require.ErrorIs(t, u.Insert(0), model.ErrOIDCIdentityUnresolved)
				require.NoError(t, json.Unmarshal([]byte(`{"oidc_issuer":"injected","oidc_identity_key":"injected"}`), &u))
				require.Empty(t, u.OidcIssuer)
				require.Nil(t, u.OidcIdentityKey)
			}
		})
	}
}
