package model_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"one-api/model"
	"one-api/relay/relay_util"
	"one-api/types"
)

func TestQuotaDurableTerminalRetryAfterDatabaseFailure(t *testing.T) {
	for _, refund := range []bool{false, true} {
		t.Run(map[bool]string{false: "consume", true: "refund"}[refund], func(t *testing.T) {
			db, c := quotaLifecycleFixture(t, false, false)
			q := relay_util.NewQuota(c, "quota-fixture", 0)
			require.Nil(t, q.PreQuotaConsumption())
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("durable_fixture_failure", func(tx *gorm.DB) {
				if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "tokens" {
					tx.AddError(errors.New("fixture unavailable"))
				}
			}))
			if refund {
				q.Undo(c)
			} else {
				q.Consume(c, &types.Usage{PromptTokens: 7}, false)
			}
			requireQuotaPair(t, db, 1, 980, 980, 20)
			require.NoError(t, db.Callback().Update().Remove("durable_fixture_failure"))
			if refund {
				q.Undo(c)
			} else {
				q.Consume(c, &types.Usage{PromptTokens: 7}, false)
			}
			spent, requests := 7, 1
			if refund {
				spent, requests = 0, 0
			}
			quotaBalances(t, db, c, spent, requests)
			q.Undo(c)
			q.Consume(c, &types.Usage{PromptTokens: 7}, false)
			quotaBalances(t, db, c, spent, requests)
		})
	}
}

func TestQuotaDurableIntentWriteFailureRecoversWithoutCaller(t *testing.T) {
	for _, refund := range []bool{false, true} {
		t.Run(map[bool]string{false: "consume", true: "refund"}[refund], func(t *testing.T) {
			db, c := quotaLifecycleFixture(t, false, false)
			q := relay_util.NewQuota(c, "quota-fixture", 0)
			require.Nil(t, q.PreQuotaConsumption())
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("intent_failure", func(tx *gorm.DB) {
				values, _ := tx.Statement.Dest.(map[string]interface{})
				if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "quota_reservations" && values["state"] == model.QuotaReservationPending {
					tx.AddError(errors.New("intent storage temporarily unavailable"))
				}
			}))
			if refund {
				q.Undo(c)
			} else {
				q.Consume(c, &types.Usage{PromptTokens: 7}, false)
			}
			q = nil
			requireQuotaPair(t, db, 1, 980, 980, 20)
			require.NoError(t, db.Callback().Update().Remove("intent_failure"))
			require.NoError(t, model.RecoverQuotaReservations(context.Background(), 100))
			require.NoError(t, model.RecoverQuotaReservations(context.Background(), 100))
			spent, requests := 7, 1
			if refund {
				spent, requests = 0, 0
			}
			quotaBalances(t, db, c, spent, requests)
		})
	}
}

func TestQuotaDurableDoesNotKeepDeletedLogCopy(t *testing.T) {
	db, c := quotaLifecycleFixture(t, false, false)
	identity := model.QuotaReservationIdentity{UserID: 1, TokenID: c.GetInt("token_id"), ChannelID: 1, ModelName: "fixture"}
	require.NoError(t, model.ReserveQuota("retention-fixture", identity, 20))
	terminal := ledgerTerminal(7)
	terminal.Log.CreatedAt = 1
	terminal.Log.SourceIp = "retention-fixture"
	require.NoError(t, model.PrepareQuotaTerminal("retention-fixture", terminal))
	require.NoError(t, model.FinalizeQuotaReservation("retention-fixture"))
	count, err := model.DeleteOldLog(2)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	var receipt model.QuotaReservation
	require.NoError(t, db.First(&receipt, "id = ?", "retention-fixture").Error)
	require.Empty(t, receipt.TerminalLog)
	require.NoError(t, model.FinalizeQuotaReservation("retention-fixture"))
	var logs int64
	require.NoError(t, db.Model(&model.Log{}).Count(&logs).Error)
	require.Zero(t, logs)
	requireQuotaPair(t, db, 1, 993, 993, 7)
}
