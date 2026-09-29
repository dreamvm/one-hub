package model_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"one-api/model"
	"one-api/relay/relay_util"
	"one-api/types"
)

func TestQuotaTransactionIdentity(t *testing.T) {
	for _, batch := range []bool{false, true} {
		for _, unlimited := range []bool{false, true} {
			for _, change := range []string{"mode", "owner"} {
				for _, during := range []bool{false, true} {
					t.Run(fmt.Sprintf("batch=%t/unlimited=%t/%s/during=%t", batch, unlimited, change, during), func(t *testing.T) {
						db, token := quotaTransactionFixture(t, batch, unlimited)
						require.NoError(t, db.Create(&model.User{Id: 2, Username: "other-owner", AccessToken: "other-fixture-access", AffCode: "other-fixture-aff", Quota: 1000}).Error)
						mutate := func() {
							column, value := "unlimited_quota", any(!unlimited)
							if change == "owner" {
								column, value = "user_id", 2
							}
							require.NoError(t, db.Model(&token).Update(column, value).Error)
						}
						if during {
							changed := false
							require.NoError(t, db.Callback().Update().Before("gorm:update").Register("identity_change", func(tx *gorm.DB) {
								if !changed && tx.Statement.Schema != nil && strings.HasSuffix(tx.Statement.Schema.Table, "_users") {
									changed = true
									mutate()
								}
							}))
						} else {
							mutate()
						}
						require.Error(t, model.PreConsumeTokenQuotaWithInfo(token.Id, 1, unlimited, 20))
						model.FlushQuotaBatchForTest()
						requireQuotaPair(t, db, token.Id, 1000, 1000, 0)
						var other model.User
						require.NoError(t, db.First(&other, 2).Error)
						require.Equal(t, 1000, other.Quota)
					})
				}
			}
		}
	}
}

func TestQuotaTransactionIdentityModeControl(t *testing.T) {
	for _, unlimited := range []bool{false, true} {
		for _, amount := range []int{0, 7, 20, 35} {
			t.Run(fmt.Sprintf("unlimited=%t/amount=%d", unlimited, amount), func(t *testing.T) {
				db, token := quotaTransactionFixture(t, true, unlimited)
				require.NoError(t, model.PreConsumeTokenQuotaWithInfo(token.Id, 1, unlimited, 20))
				require.NoError(t, db.Model(&token).Update("unlimited_quota", !unlimited).Error)
				require.NoError(t, model.PostConsumeTokenQuotaWithInfo(token.Id, 1, unlimited, amount-20))
				tokenSpent := amount
				if unlimited {
					tokenSpent = 0
				}
				model.FlushQuotaBatchForTest()
				requireQuotaPair(t, db, token.Id, 1000-amount, 1000-tokenSpent, tokenSpent)
			})
		}
	}
}

func TestQuotaIdentityRejectsChangedAuthentication(t *testing.T) {
	for _, batch := range []bool{false, true} {
		for _, unlimited := range []bool{false, true} {
			for _, change := range []string{"mode", "owner"} {
				t.Run(fmt.Sprintf("batch=%t/unlimited=%t/%s", batch, unlimited, change), func(t *testing.T) {
					db, c := quotaLifecycleFixture(t, batch, unlimited, true)
					require.NoError(t, db.Create(&model.User{Id: 2, Username: "other-owner", AccessToken: "other-fixture-access", AffCode: "other-fixture-aff", Quota: 1000}).Error)
					q := relay_util.NewQuota(c, "quota-fixture", 0)
					column, value := "unlimited_quota", any(!unlimited)
					if change == "owner" {
						column, value = "user_id", 2
					}
					require.NoError(t, db.Model(&model.Token{}).Where("id = ?", c.GetInt("token_id")).Update(column, value).Error)
					err := q.PreQuotaConsumption()
					require.NotNil(t, err, "a reservation cannot change the authenticated accounting identity")
					require.Equal(t, http.StatusForbidden, err.StatusCode)
					require.Equal(t, "pre_consume_token_quota_failed", err.Code)
					require.False(t, q.HandelStatus)
					q.Undo(c)
					model.FlushQuotaBatchForTest()
					requireQuotaPair(t, db, c.GetInt("token_id"), 1000, 1000, 0)
					var other model.User
					require.NoError(t, db.First(&other, 2).Error)
					require.Equal(t, 1000, other.Quota)
					// A newly authenticated context with current identity still works.
					if change == "mode" {
						c.Set("token_unlimited_quota", !unlimited)
					} else {
						c.Set("id", 2)
					}
					next := relay_util.NewQuota(c, "quota-fixture", 0)
					require.Nil(t, next.PreQuotaConsumption())
					next.Undo(c)
				})
			}
		}
	}
}

func TestQuotaIdentitySettlementKeepsReservedMode(t *testing.T) {
	for _, unlimited := range []bool{false, true} {
		for _, amount := range []int{-1, 0, 7, 20, 35} {
			t.Run(fmt.Sprintf("unlimited=%t/amount=%d", unlimited, amount), func(t *testing.T) {
				db, c := quotaLifecycleFixture(t, true, unlimited, true)
				q := relay_util.NewQuota(c, "quota-fixture", 0)
				require.Nil(t, q.PreQuotaConsumption())
				require.NoError(t, db.Model(&model.Token{}).Where("id = ?", c.GetInt("token_id")).Update("unlimited_quota", !unlimited).Error)
				if amount == -1 {
					q.Undo(c)
					model.FlushQuotaBatchForTest()
					quotaBalances(t, db, c, 0, 0)
				} else {
					q.Consume(c, &types.Usage{PromptTokens: amount}, false)
					model.FlushQuotaBatchForTest()
					quotaBalances(t, db, c, amount, 1)
				}
			})
		}
	}
}
