package model_test

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"one-api/model"
	"one-api/payment"
)

func TestQuotaTransactionPaymentLateLifecycle(t *testing.T) {
	for _, stage := range []string{"normal", "closed", "disabled", "deleted gateway", "closed disabled", "closed deleted", "bad amount", "bad signature", "wrong gateway", "settled marker", "log failure"} {
		t.Run(stage, func(t *testing.T) {
			db, gateway, order, client := paymentTransactionFixture(t, true)
			if stage != "normal" && stage != "disabled" && stage != "deleted gateway" {
				require.NoError(t, db.Model(&order).Update("created_at", time.Now().Add(-4*time.Hour).Unix()).Error)
				require.NoError(t, model.CloseUnfinishedOrder())
			}
			if strings.Contains(stage, "disabled") {
				require.NoError(t, db.Model(&gateway).Update("enable", false).Error)
			}
			if strings.Contains(stage, "deleted") {
				require.NoError(t, gateway.Delete())
			}
			if strings.Contains(stage, "disabled") || strings.Contains(stage, "deleted") {
				_, err := payment.NewPaymentService(gateway.UUID)
				require.Error(t, err, "retired gateway must not create new payments")
			}
			switch stage {
			case "bad signature":
				client.Key = "different-fixture-signature"
			case "wrong gateway":
				require.NoError(t, db.Model(&order).Update("gateway_id", gateway.ID+1).Error)
			case "settled marker":
				require.NoError(t, db.Model(&order).Update("settled_at", 1).Error)
			case "log failure":
				require.NoError(t, db.Callback().Create().Before("gorm:create").Register("late_log_fault", func(tx *gorm.DB) {
					if tx.Statement.Schema != nil && strings.HasSuffix(tx.Statement.Schema.Table, "_logs") {
						tx.AddError(errors.New("late log failure"))
					}
				}))
				t.Cleanup(func() { require.NoError(t, db.Callback().Create().Remove("late_log_fault")) })
			}
			facts := map[string]string{}
			if stage == "bad amount" {
				facts["money"] = "6.00"
			}
			response := paymentCallbackWithFacts(gateway, order, client, "late-transaction", facts)
			reject := stage == "bad amount" || stage == "bad signature" || stage == "wrong gateway" || stage == "settled marker" || stage == "log failure"
			if reject {
				require.NotEqual(t, "success", response.Body.String())
				requireQuotaPair(t, db, 1, 1000, 1000, 0)
				return
			}
			require.Equal(t, "success", response.Body.String())
			require.Equal(t, "success", paymentCallbackFixture(gateway, order, client, "late-transaction").Body.String())
			require.NoError(t, model.CloseUnfinishedOrder())
			saved, err := model.GetOrderByTradeNo(order.TradeNo)
			require.NoError(t, err)
			require.Equal(t, model.OrderStatusSuccess, saved.Status)
			requireQuotaPair(t, db, 1, 1100, 1000, 0)
			var logs int64
			require.NoError(t, db.Model(&model.Log{}).Count(&logs).Error)
			require.EqualValues(t, 1, logs)
		})
	}
}

func TestQuotaTransactionPaymentCloseRace(t *testing.T) {
	db, gateway, order, client := paymentTransactionFixture(t, false)
	require.NoError(t, db.Model(&order).Update("created_at", time.Now().Add(-4*time.Hour).Unix()).Error)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = model.CloseUnfinishedOrder()
			paymentCallbackFixture(gateway, order, client, "race-late")
		}()
	}
	wg.Wait()
	require.Equal(t, "success", paymentCallbackFixture(gateway, order, client, "race-late").Body.String())
	require.NoError(t, model.CloseUnfinishedOrder())
	saved, err := model.GetOrderByTradeNo(order.TradeNo)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusSuccess, saved.Status)
	requireQuotaPair(t, db, 1, 1100, 1000, 0)
}
