package model_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"one-api/model"
)

func compensationFixture(t *testing.T, batch, unlimited bool, amount int) (*gorm.DB, model.QuotaReservationIdentity, model.Task) {
	t.Helper()
	db, identity := quotaLedgerFixture(t, batch, unlimited)
	for _, table := range []any{&model.Task{}, &model.Midjourney{}} {
		require.NoError(t, db.AutoMigrate(table))
	}
	t.Cleanup(func() { require.NoError(t, db.Migrator().DropTable(&model.Task{}, &model.Midjourney{})) })
	require.NoError(t, model.ReserveQuota("compensation-fixture", identity, 20))
	require.NoError(t, model.PrepareQuotaTerminal("compensation-fixture", ledgerTerminal(amount)))
	require.NoError(t, model.FinalizeQuotaReservation("compensation-fixture"))
	task := model.Task{UserId: identity.UserID, TokenID: identity.TokenID, ChannelId: identity.ChannelID, TaskID: "provider-task", Platform: model.TaskPlatformSuno, Status: model.TaskStatusInProgress, Progress: 10, Quota: 999, ReservationID: "compensation-fixture"}
	require.NoError(t, task.Insert())
	return db, identity, task
}

func requireCompensated(t *testing.T, db *gorm.DB, identity model.QuotaReservationIdentity, amount int) {
	t.Helper()
	requireQuotaPair(t, db, identity.TokenID, 1000, 1000, 0)
	var user model.User
	var channel model.Channel
	var receipt model.QuotaReservation
	var logs int64
	require.NoError(t, db.First(&user, identity.UserID).Error)
	require.NoError(t, db.First(&channel, identity.ChannelID).Error)
	require.Equal(t, 1, user.RequestCount)
	require.EqualValues(t, amount, user.UsedQuota)
	require.EqualValues(t, amount, channel.UsedQuota)
	require.NoError(t, db.First(&receipt, "id = ?", "compensation-fixture").Error)
	require.Equal(t, model.QuotaReservationConsumed, receipt.State)
	require.Equal(t, model.QuotaCompensationApplied, receipt.CompensationState)
	require.NoError(t, db.Model(&model.Log{}).Where("type = ?", model.LogTypeSystem).Count(&logs).Error)
	expected := 1
	if amount == 0 {
		expected = 0
	}
	require.EqualValues(t, expected, logs)
}

func TestQuotaTransactionCompensationReplayAndControls(t *testing.T) {
	for _, batch := range []bool{false, true} {
		for _, unlimited := range []bool{false, true} {
			for _, amount := range []int{0, 7, 35} {
				t.Run(fmt.Sprintf("batch=%t/unlimited=%t/amount=%d", batch, unlimited, amount), func(t *testing.T) {
					db, identity, original := compensationFixture(t, batch, unlimited, amount)
					// Later admin mode changes do not change the original accounting pair.
					require.NoError(t, db.Model(&model.Token{}).Where("id = ?", identity.TokenID).Update("unlimited_quota", !unlimited).Error)
					first := original
					first.Status = model.TaskStatusFailure
					first.Progress = 100
					require.NoError(t, first.UpdateFromPoll(true))
					stale := original
					stale.Status = model.TaskStatusSuccess
					stale.Progress = 100
					require.NoError(t, stale.UpdateFromPoll(false))
					require.NoError(t, original.UpdateFromPoll(true))
					// A duplicated local task with the same receipt cannot refund twice.
					duplicate := original
					duplicate.ID = 0
					require.NoError(t, duplicate.Insert())
					require.NoError(t, duplicate.UpdateFromPoll(true))
					require.NoError(t, model.RecoverQuotaCompensations(context.Background(), 100))
					model.FlushQuotaBatchForTest()
					requireCompensated(t, db, identity, amount)
					var saved model.Task
					require.NoError(t, db.First(&saved, original.ID).Error)
					require.Equal(t, model.TaskStatus(model.TaskStatusFailure), saved.Status)
					require.Equal(t, 100, saved.Progress)
				})
			}
		}
	}
}

func TestQuotaTransactionCompensationAtomicFailure(t *testing.T) {
	for _, stage := range []string{"intent", "task", "user", "token", "log", "applied"} {
		t.Run(stage, func(t *testing.T) {
			db, identity, task := compensationFixture(t, true, false, 7)
			inject := func(tx *gorm.DB) {
				if tx.Statement.Schema == nil {
					return
				}
				table := tx.Statement.Schema.Table
				values, _ := tx.Statement.Dest.(map[string]any)
				fail := stage == "intent" && values["compensation_state"] == model.QuotaCompensationPending || stage == "task" && strings.HasSuffix(table, "_tasks") && values["refund_status"] != nil || stage == "user" && strings.HasSuffix(table, "_users") || stage == "token" && strings.HasSuffix(table, "_tokens") || stage == "log" && strings.HasSuffix(table, "_logs") || stage == "applied" && values["compensation_state"] == model.QuotaCompensationApplied
				if fail {
					tx.AddError(errors.New("injected compensation write failure"))
				}
			}
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("compensation_fault", inject))
			require.NoError(t, db.Callback().Create().Before("gorm:create").Register("compensation_fault", inject))
			require.Error(t, task.UpdateFromPoll(true))
			requireQuotaPair(t, db, identity.TokenID, 993, 993, 7)
			var saved model.Task
			var receipt model.QuotaReservation
			require.NoError(t, db.First(&saved, task.ID).Error)
			require.NoError(t, db.First(&receipt, "id = ?", task.ReservationID).Error)
			if stage == "intent" || stage == "task" {
				require.Equal(t, 10, saved.Progress)
				require.Empty(t, receipt.CompensationState)
			} else {
				require.Equal(t, 100, saved.Progress)
				require.Equal(t, model.QuotaCompensationPending, receipt.CompensationState)
			}
			require.NoError(t, db.Callback().Update().Remove("compensation_fault"))
			require.NoError(t, db.Callback().Create().Remove("compensation_fault"))
			// A fresh database session and persisted intent suffice; no old request object.
			if saved.Progress != 100 {
				require.NoError(t, saved.UpdateFromPoll(true))
			}
			fresh, err := gorm.Open(db.Dialector, &gorm.Config{NamingStrategy: db.NamingStrategy})
			require.NoError(t, err)
			model.DB = fresh
			t.Cleanup(func() {
				model.DB = db
				sqlDB, e := fresh.DB()
				require.NoError(t, e)
				require.NoError(t, sqlDB.Close())
			})
			require.NoError(t, model.RecoverQuotaCompensations(context.Background(), 100))
			require.NoError(t, model.RecoverQuotaCompensations(context.Background(), 100))
			requireCompensated(t, db, identity, 7)
		})
	}
}

func TestQuotaTransactionCompensationConcurrent(t *testing.T) {
	db, identity, task := compensationFixture(t, true, false, 7)
	var wg sync.WaitGroup
	results := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); copy := task; results <- copy.UpdateFromPoll(true) }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	require.NoError(t, model.RecoverQuotaCompensations(context.Background(), 100))
	requireCompensated(t, db, identity, 7)
}

func TestQuotaTransactionCompensationPendingAndUncharged(t *testing.T) {
	for _, outcome := range []string{model.QuotaOutcomeConsume, model.QuotaOutcomeRefund} {
		t.Run(outcome, func(t *testing.T) {
			db, identity := quotaLedgerFixture(t, false, false)
			require.NoError(t, db.AutoMigrate(&model.Task{}))
			t.Cleanup(func() { require.NoError(t, db.Migrator().DropTable(&model.Task{})) })
			require.NoError(t, model.ReserveQuota("compensation-fixture", identity, 20))
			task := model.Task{UserId: 1, TokenID: identity.TokenID, ChannelId: 1, TaskID: "pending", Platform: model.TaskPlatformSuno, ReservationID: "compensation-fixture"}
			require.NoError(t, task.Insert())
			require.Error(t, task.UpdateFromPoll(true))
			requireQuotaPair(t, db, identity.TokenID, 980, 980, 20)
			terminal := ledgerTerminal(7)
			if outcome == model.QuotaOutcomeRefund {
				terminal = model.QuotaTerminal{Outcome: outcome}
			}
			require.NoError(t, model.PrepareQuotaTerminal(task.ReservationID, terminal))
			require.NoError(t, model.RecoverQuotaReservations(context.Background(), 100))
			requireQuotaPair(t, db, identity.TokenID, 1000, 1000, 0)
			var receipt model.QuotaReservation
			require.NoError(t, db.First(&receipt, "id = ?", task.ReservationID).Error)
			expected := model.QuotaCompensationApplied
			if outcome == model.QuotaOutcomeRefund {
				expected = model.QuotaCompensationNotCharged
			}
			require.Equal(t, expected, receipt.CompensationState)
		})
	}
}

func TestQuotaTransactionCompensationUncertainIdentity(t *testing.T) {
	for _, mode := range []string{"legacy", "missing receipt", "wrong owner", "wrong channel", "moved token", "deleted token", "overflow"} {
		t.Run(mode, func(t *testing.T) {
			db, identity, task := compensationFixture(t, false, false, 7)
			switch mode {
			case "legacy":
				task.ReservationID = ""
				require.NoError(t, db.Model(&task).Update("reservation_id", "").Error)
			case "missing receipt":
				task.ReservationID = "missing"
				require.NoError(t, db.Model(&task).Update("reservation_id", "missing").Error)
			case "wrong owner":
				task.UserId = 2
				require.NoError(t, db.Model(&task).Update("user_id", 2).Error)
			case "wrong channel":
				task.ChannelId = 2
				require.NoError(t, db.Model(&task).Update("channel_id", 2).Error)
			case "moved token":
				require.NoError(t, db.Model(&model.Token{}).Where("id = ?", identity.TokenID).Update("user_id", 2).Error)
			case "deleted token":
				require.NoError(t, db.Delete(&model.Token{}, identity.TokenID).Error)
			case "overflow":
				require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("quota", int(^uint(0)>>1)).Error)
			}
			err := task.UpdateFromPoll(true)
			if mode == "moved token" || mode == "deleted token" || mode == "overflow" {
				require.Error(t, err)
				require.Error(t, model.RecoverQuotaCompensations(context.Background(), 100))
			} else {
				require.NoError(t, err)
				var saved model.Task
				require.NoError(t, db.First(&saved, task.ID).Error)
				require.Equal(t, model.TaskRefundReviewRequired, saved.RefundStatus)
			}
			userQuota := 993
			if mode == "overflow" {
				userQuota = int(^uint(0) >> 1)
			}
			requireQuotaPair(t, db, identity.TokenID, userQuota, 993, 7)
		})
	}
}

func TestQuotaTransactionCompensationMJTerminalAndBinding(t *testing.T) {
	for _, outcome := range []string{"failure", "success", "progress"} {
		t.Run(outcome, func(t *testing.T) {
			db, identity, original := compensationFixture(t, false, false, 7)
			mj := model.Midjourney{UserId: 1, TokenID: identity.TokenID, ChannelId: 1, MjId: original.TaskID, ReservationID: original.ReservationID, Progress: "10%", Status: "IN_PROGRESS"}
			require.NoError(t, mj.Insert())
			next := mj
			if outcome == "failure" {
				next.Status = "FAILURE"
				next.Progress = "100%"
			}
			if outcome == "success" {
				next.Status = "SUCCESS"
				next.Progress = "100%"
			}
			require.NoError(t, next.UpdateFromPoll(outcome == "failure"))
			if outcome == "failure" {
				requireCompensated(t, db, identity, 7)
			} else {
				requireLedgerAccounting(t, db, identity, 7, 1)
			}
			if outcome != "progress" {
				require.NoError(t, mj.UpdateFromPoll(true))
				require.NoError(t, model.MjBulkUpdateByTaskIds([]int{mj.Id}, map[string]any{"progress": "0%"}))
				var saved model.Midjourney
				require.NoError(t, db.First(&saved, mj.Id).Error)
				require.Equal(t, next.Status, saved.Status)
				require.Equal(t, "100%", saved.Progress)
			}
			// Same external ID on another channel must never acquire this refund intent.
			other := original
			other.ID = 0
			other.ChannelId = 2
			require.NoError(t, other.Insert())
			require.NoError(t, model.TaskBulkUpdateForChannel(model.TaskPlatformSuno, 1, []string{original.TaskID}, map[string]any{"progress": 100, "status": "FAILURE"}))
			var saved model.Task
			require.NoError(t, db.First(&saved, other.ID).Error)
			require.Equal(t, 10, saved.Progress)
			changed := other
			changed.UserId = 2
			require.Error(t, changed.UpdateFromPoll(true))
		})
	}
}
