package model_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	stripeapi "github.com/stripe/stripe-go/v80"
	"github.com/stripe/stripe-go/v80/webhook"
	"gorm.io/gorm"

	"one-api/controller"
	"one-api/model"
	"one-api/payment/gateway/epay"
)

func paymentTransactionFixture(t *testing.T, batch bool) (*gorm.DB, model.Payment, model.Order, *epay.Client) {
	t.Helper()
	db, _ := redemptionFixture(t, batch)
	for _, table := range []any{&model.Payment{}, &model.Order{}, &model.OrderPaymentClaim{}} {
		require.NoError(t, db.AutoMigrate(table))
	}
	t.Cleanup(func() {
		require.NoError(t, db.Migrator().DropTable(&model.OrderPaymentClaim{}, &model.Order{}, &model.Payment{}))
	})
	client := &epay.Client{PartnerID: "fixture-merchant", Key: "fixture-payment-signing-only"}
	conf, err := json.Marshal(epay.EpayConfig{Client: *client, PayType: epay.Alipay})
	require.NoError(t, err)
	enabled := true
	gateway := model.Payment{Type: "epay", UUID: "payment-fixture", Name: "fixture", Currency: model.CurrencyTypeCNY, Config: string(conf), Enable: &enabled}
	require.NoError(t, db.Create(&gateway).Error)
	order := model.Order{UserId: 1, GatewayId: gateway.ID, TradeNo: "payment-fixture-order", Quota: 100, Amount: 1, OrderAmount: 7, OrderCurrency: model.CurrencyTypeCNY, Status: model.OrderStatusPending}
	require.NoError(t, order.Insert())
	return db, gateway, order, client
}

func paymentCallbackFixture(gateway model.Payment, order model.Order, client *epay.Client, gatewayNo string) *httptest.ResponseRecorder {
	return paymentCallbackWithFacts(gateway, order, client, gatewayNo, nil)
}

func paymentCallbackWithFacts(gateway model.Payment, order model.Order, client *epay.Client, gatewayNo string, overrides map[string]string) *httptest.ResponseRecorder {
	fields := map[string]string{"pid": client.PartnerID, "out_trade_no": order.TradeNo, "trade_no": gatewayNo, "money": "7.00", "type": "alipay", "trade_status": epay.TradeStatusSuccess}
	for k, v := range overrides {
		fields[k] = v
	}
	fields["sign"] = client.Sign(fields)
	query := url.Values{}
	for k, v := range fields {
		query.Set(k, v)
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "uuid", Value: gateway.UUID}}
	c.Request = httptest.NewRequest("GET", "/api/payment/notify/"+gateway.UUID+"?"+query.Encode(), nil)
	controller.PaymentCallback(c)
	return recorder
}

func TestQuotaTransactionPaymentCallbackAtomic(t *testing.T) {
	for _, stage := range []string{"normal", "batch", "missing user", "log failure"} {
		t.Run(stage, func(t *testing.T) {
			db, gateway, order, client := paymentTransactionFixture(t, stage == "batch")
			if stage == "missing user" {
				require.NoError(t, db.Delete(&model.User{}, 1).Error)
			}
			if stage == "log failure" {
				require.NoError(t, db.Callback().Create().Before("gorm:create").Register("payment_log_fault", func(tx *gorm.DB) {
					if tx.Statement.Schema != nil && strings.HasSuffix(tx.Statement.Schema.Table, "_logs") {
						tx.AddError(errors.New("fixture log failure"))
					}
				}))
				t.Cleanup(func() { require.NoError(t, db.Callback().Create().Remove("payment_log_fault")) })
			}
			response := paymentCallbackFixture(gateway, order, client, "payment-transaction")
			var saved model.Order
			require.NoError(t, db.First(&saved, order.ID).Error)
			if stage == "missing user" || stage == "log failure" {
				require.NotEqual(t, "success", response.Body.String(), "provider must retry uncommitted recharge")
				require.Equal(t, model.OrderStatusPending, saved.Status)
				return
			}
			require.Equal(t, "success", response.Body.String())
			require.Equal(t, model.OrderStatusSuccess, saved.Status)
			requireQuotaPair(t, db, 1, 1100, 1000, 0)
			require.Equal(t, "success", paymentCallbackFixture(gateway, order, client, "payment-transaction").Body.String())
			model.FlushQuotaBatchForTest()
			requireQuotaPair(t, db, 1, 1100, 1000, 0)
			var logs int64
			require.NoError(t, db.Model(&model.Log{}).Count(&logs).Error)
			require.EqualValues(t, 1, logs)
		})
	}
}

func TestQuotaTransactionPaymentConcurrent(t *testing.T) {
	db, gateway, order, client := paymentTransactionFixture(t, true)
	var wg sync.WaitGroup
	responses := make(chan string, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			responses <- paymentCallbackFixture(gateway, order, client, "payment-transaction").Body.String()
		}()
	}
	wg.Wait()
	close(responses)
	for response := range responses {
		require.Equal(t, "success", response)
	}
	model.FlushQuotaBatchForTest()
	requireQuotaPair(t, db, 1, 1100, 1000, 0)
	var logs int64
	require.NoError(t, db.Model(&model.Log{}).Count(&logs).Error)
	require.EqualValues(t, 1, logs)
}

func TestQuotaTransactionPaymentWrongGateway(t *testing.T) {
	db, gateway, order, client := paymentTransactionFixture(t, false)
	require.NoError(t, db.Model(&order).Update("gateway_id", gateway.ID+1).Error)
	require.NotEqual(t, "success", paymentCallbackFixture(gateway, order, client, "payment-transaction").Body.String())
	requireQuotaPair(t, db, 1, 1000, 1000, 0)
}

func TestQuotaTransactionPaymentEqualOrders(t *testing.T) {
	db, gateway, order, client := paymentTransactionFixture(t, true)
	for i := 0; i < 2; i++ {
		if i > 0 {
			order.ID = 0
			order.TradeNo = fmt.Sprintf("another-%d", i)
			require.NoError(t, order.Insert())
		}
		require.Equal(t, "success", paymentCallbackFixture(gateway, order, client, fmt.Sprintf("transaction-%d", i)).Body.String())
	}
	requireQuotaPair(t, db, 1, 1200, 1000, 0)
}

func TestQuotaTransactionPaymentRollbackAndReplay(t *testing.T) {
	for _, stage := range []string{"lock", "claim", "balance", "promotion", "log", "terminal"} {
		t.Run(stage, func(t *testing.T) {
			db, gateway, order, client := paymentTransactionFixture(t, true)
			inject := func(tx *gorm.DB) {
				if tx.Statement.Schema == nil {
					return
				}
				table := tx.Statement.Schema.Table
				fields, _ := tx.Statement.Dest.(map[string]interface{})
				if stage == "claim" && strings.HasSuffix(table, "_order_payment_claims") || stage == "lock" && strings.HasSuffix(table, "_orders") || stage == "balance" && strings.HasSuffix(table, "_users") || stage == "promotion" && strings.HasSuffix(table, "_user_groups") || stage == "log" && strings.HasSuffix(table, "_logs") || stage == "terminal" && strings.HasSuffix(table, "_orders") && fields["status"] == model.OrderStatusSuccess {
					tx.AddError(errors.New("injected payment failure"))
				}
			}
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("payment_fault", inject))
			require.NoError(t, db.Callback().Create().Before("gorm:create").Register("payment_fault", inject))
			require.NoError(t, db.Callback().Query().Before("gorm:query").Register("payment_fault", inject))
			require.NotEqual(t, "success", paymentCallbackFixture(gateway, order, client, "payment-transaction").Body.String())
			require.NoError(t, db.Callback().Update().Remove("payment_fault"))
			require.NoError(t, db.Callback().Create().Remove("payment_fault"))
			require.NoError(t, db.Callback().Query().Remove("payment_fault"))
			var saved model.Order
			require.NoError(t, db.First(&saved, order.ID).Error)
			require.Equal(t, model.OrderStatusPending, saved.Status)
			require.Zero(t, saved.SettledAt)
			requireQuotaPair(t, db, 1, 1000, 1000, 0)
			var logs int64
			require.NoError(t, db.Model(&model.Log{}).Count(&logs).Error)
			require.Zero(t, logs)
			reopened, err := gorm.Open(db.Dialector, &gorm.Config{NamingStrategy: db.NamingStrategy})
			require.NoError(t, err)
			sqlDB, err := reopened.DB()
			require.NoError(t, err)
			model.DB = reopened
			response := paymentCallbackFixture(gateway, order, client, "payment-transaction")
			model.DB = db
			require.NoError(t, sqlDB.Close())
			require.Equal(t, "success", response.Body.String())
			require.Equal(t, "success", paymentCallbackFixture(gateway, order, client, "payment-transaction").Body.String())
			require.NotEqual(t, "success", paymentCallbackFixture(gateway, order, client, "different-transaction").Body.String())
			model.FlushQuotaBatchForTest()
			requireQuotaPair(t, db, 1, 1100, 1000, 0)
			require.NoError(t, db.Model(&model.Log{}).Count(&logs).Error)
			require.EqualValues(t, 1, logs)
		})
	}
}

func TestQuotaTransactionPaymentInvalidState(t *testing.T) {
	for _, state := range []string{"zero", "negative", "overflow", "legacy success", "failed", "deleted order", "empty reference"} {
		t.Run(state, func(t *testing.T) {
			db, gateway, order, client := paymentTransactionFixture(t, false)
			expected := 1000
			switch state {
			case "zero":
				require.NoError(t, db.Model(&order).Update("quota", 0).Error)
			case "negative":
				require.NoError(t, db.Model(&order).Update("quota", -1).Error)
			case "overflow":
				expected = int(^uint(0)>>1) - 10
				require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("quota", expected).Error)
			case "legacy success":
				require.NoError(t, db.Model(&order).Updates(map[string]any{"status": model.OrderStatusSuccess, "gateway_no": "payment-transaction"}).Error)
			case "failed":
				require.NoError(t, db.Model(&order).Update("status", model.OrderStatusFailed).Error)
			case "deleted order":
				require.NoError(t, db.Delete(&order).Error)
			}
			reference := "payment-transaction"
			if state == "empty reference" {
				reference = ""
			}
			require.NotEqual(t, "success", paymentCallbackFixture(gateway, order, client, reference).Body.String())
			requireQuotaPair(t, db, 1, expected, 1000, 0)
		})
	}
}

func TestQuotaTransactionPaymentStripeAcknowledgement(t *testing.T) {
	for _, state := range []string{"ignored", "invalid signature", "missing intent", "normal", "log failure"} {
		t.Run(state, func(t *testing.T) {
			db, gateway, order, _ := paymentTransactionFixture(t, true)
			gateway.Type = "stripe"
			gateway.Config = `{"webhook_secret":"fixture-webhook-signing-only"}`
			require.NoError(t, db.Model(&gateway).Updates(map[string]any{"type": gateway.Type, "config": gateway.Config}).Error)
			kind := "checkout.session.completed"
			if state == "ignored" {
				kind = "payment_intent.created"
			}
			intent := `"payment-intent-fixture"`
			if state == "missing intent" {
				intent = "null"
			}
			body := fmt.Sprintf(`{"id":"event-fixture","object":"event","api_version":%q,"type":%q,"data":{"object":{"client_reference_id":%q,"payment_intent":%s,"payment_status":"paid","amount_total":700,"currency":"cny","metadata":{"user_id":"1"}}}}`, stripeapi.APIVersion, kind, order.TradeNo, intent)
			signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: []byte(body), Secret: "fixture-webhook-signing-only"})
			if state == "log failure" {
				require.NoError(t, db.Callback().Create().Before("gorm:create").Register("stripe_log_fault", func(tx *gorm.DB) {
					if tx.Statement.Schema != nil && strings.HasSuffix(tx.Statement.Schema.Table, "_logs") {
						tx.AddError(errors.New("fixture log failure"))
					}
				}))
				t.Cleanup(func() { require.NoError(t, db.Callback().Create().Remove("stripe_log_fault")) })
			}
			response := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(response)
			c.Params = gin.Params{{Key: "uuid", Value: gateway.UUID}}
			c.Request = httptest.NewRequest("POST", "/api/payment/notify/"+gateway.UUID, strings.NewReader(body))
			c.Request.Header.Set("Stripe-Signature", signed.Header)
			if state == "invalid signature" {
				c.Request.Header.Set("Stripe-Signature", "invalid")
			}
			controller.PaymentCallback(c)
			c.Writer.WriteHeaderNow()
			expected := 1000
			if state == "normal" {
				expected = 1100
				require.Equal(t, 200, response.Code)
			} else if state == "ignored" {
				require.Equal(t, 200, response.Code)
			} else {
				require.GreaterOrEqual(t, response.Code, 500)
			}
			requireQuotaPair(t, db, 1, expected, 1000, 0)
		})
	}
}
