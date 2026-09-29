package model_test

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"one-api/payment/gateway/epay"

	"github.com/stretchr/testify/require"
	stripeapi "github.com/stripe/stripe-go/v80"
	"github.com/stripe/stripe-go/v80/webhook"

	"one-api/model"
)

func TestQuotaTransactionPaymentVerifiedFacts(t *testing.T) {
	for _, test := range []struct {
		name   string
		values map[string]string
		valid  bool
	}{
		{"normal", nil, true}, {"coupon total", map[string]string{"money": "7", "buyer_pay_amount": "5.00"}, true},
		{"underpayment", map[string]string{"money": "0.01"}, false}, {"overpayment", map[string]string{"money": "8.00"}, false},
		{"missing amount", map[string]string{"money": ""}, false}, {"fraction", map[string]string{"money": "7.001"}, false},
		{"merchant", map[string]string{"pid": "another-merchant"}, false}, {"method", map[string]string{"type": "wxpay"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, gateway, order, client := paymentTransactionFixture(t, false)
			response := paymentCallbackWithFacts(gateway, order, client, "facts-transaction", test.values)
			if test.valid {
				require.Equal(t, "success", response.Body.String())
				requireQuotaPair(t, db, 1, 1100, 1000, 0)
			} else {
				require.NotEqual(t, "success", response.Body.String())
				requireQuotaPair(t, db, 1, 1000, 1000, 0)
			}
		})
	}
}

func TestQuotaTransactionPaymentTransactionReuse(t *testing.T) {
	db, gateway, order, client := paymentTransactionFixture(t, false)
	require.Equal(t, "success", paymentCallbackFixture(gateway, order, client, "same-transaction").Body.String())
	order.ID = 0
	order.TradeNo = "another-facts-order"
	require.NoError(t, db.Create(&order).Error)
	require.NotEqual(t, "success", paymentCallbackFixture(gateway, order, client, "same-transaction").Body.String())
	requireQuotaPair(t, db, 1, 1100, 1000, 0)
	var saved model.Order
	require.NoError(t, db.First(&saved, order.ID).Error)
	require.Equal(t, model.OrderStatusPending, saved.Status)
}

func TestQuotaTransactionPaymentStripeFacts(t *testing.T) {
	for _, state := range []string{"normal", "delayed paid", "unpaid", "underpayment", "currency", "user", "missing user"} {
		t.Run(state, func(t *testing.T) {
			db, gateway, order, _ := paymentTransactionFixture(t, false)
			gateway.Type = "stripe"
			gateway.Config = `{"webhook_secret":"fixture-webhook-signing-only"}`
			require.NoError(t, db.Model(&gateway).Updates(map[string]any{"type": gateway.Type, "config": gateway.Config}).Error)
			kind := "checkout.session.completed"
			paid := "paid"
			amount := 700
			currency := "cny"
			user := "1"
			switch state {
			case "delayed paid":
				kind = "checkout.session.async_payment_succeeded"
			case "unpaid":
				paid = "unpaid"
			case "underpayment":
				amount = 1
			case "currency":
				currency = "usd"
			case "user":
				user = "2"
			case "missing user":
				user = ""
			}
			body := fmt.Sprintf(`{"id":"facts-event","object":"event","api_version":%q,"type":%q,"data":{"object":{"client_reference_id":%q,"payment_intent":"facts-payment","payment_status":%q,"amount_total":%d,"currency":%q,"metadata":{"user_id":%q}}}}`, stripeapi.APIVersion, kind, order.TradeNo, paid, amount, currency, user)
			signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: []byte(body), Secret: "fixture-webhook-signing-only"})
			request := httptest.NewRequest("POST", "/callback", strings.NewReader(body))
			request.Header.Set("Stripe-Signature", signed.Header)
			response := runPaymentRequest(gateway, request)
			if state == "normal" || state == "delayed paid" {
				require.Equal(t, 200, response.Code)
				requireQuotaPair(t, db, 1, 1100, 1000, 0)
			} else {
				requireQuotaPair(t, db, 1, 1000, 1000, 0)
				if state == "unpaid" {
					require.Equal(t, 200, response.Code)
				} else {
					require.Equal(t, 503, response.Code)
				}
			}
		})
	}
}

func TestPaymentAmountRepresentations(t *testing.T) {
	for _, value := range []string{"", "0", "-1", "+7", " 7", "7 ", "7e0", "7.000", "7.", ".7", "NaN", "Inf", "100000000.00", "7,00", "7.0.0"} {
		t.Run(value, func(t *testing.T) { _, err := model.ParsePaymentMinor(value); require.Error(t, err) })
	}
	for value, want := range map[string]int64{"7": 700, "7.0": 700, "7.00": 700, "0.01": 1, "99999999.99": 9999999999} {
		got, err := model.ParsePaymentMinor(value)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), -1, 0, 0.001, 1.001, 100000000} {
		_, err := model.PaymentMinorFromAmount(value)
		require.Error(t, err)
	}
	for _, value := range []float64{0.01, 7.00, 99999999.99} {
		_, err := model.PaymentMinorFromAmount(value)
		require.NoError(t, err)
	}
}

func TestQuotaTransactionPaymentClaimConcurrency(t *testing.T) {
	db, gateway, first, client := paymentTransactionFixture(t, false)
	second := first
	second.ID = 0
	second.TradeNo = "concurrent-other-order"
	require.NoError(t, db.Create(&second).Error)
	var wg sync.WaitGroup
	results := make(chan string, 2)
	for _, order := range []model.Order{first, second} {
		wg.Add(1)
		go func(order model.Order) {
			defer wg.Done()
			results <- paymentCallbackFixture(gateway, order, client, "concurrent-same-transaction").Body.String()
		}(order)
	}
	wg.Wait()
	close(results)
	success := 0
	for response := range results {
		if response == "success" {
			success++
		}
	}
	require.Equal(t, 1, success)
	requireQuotaPair(t, db, 1, 1100, 1000, 0)
	var claims int64
	require.NoError(t, db.Model(&model.OrderPaymentClaim{}).Count(&claims).Error)
	require.EqualValues(t, 1, claims)
}

func TestQuotaTransactionPaymentMissingSigningSecret(t *testing.T) {
	for _, kind := range []string{"epay", "stripe"} {
		t.Run(kind, func(t *testing.T) {
			db, gateway, order, client := paymentTransactionFixture(t, false)
			gateway.Type = kind
			var response *httptest.ResponseRecorder
			if kind == "epay" {
				client.Key = ""
				conf, err := json.Marshal(epay.EpayConfig{Client: *client, PayType: epay.Alipay})
				require.NoError(t, err)
				gateway.Config = string(conf)
				require.NoError(t, db.Model(&gateway).Updates(map[string]any{"config": gateway.Config}).Error)
				response = paymentCallbackFixture(gateway, order, client, "unsigned-fixture")
				require.NotEqual(t, "success", response.Body.String())
			} else {
				gateway.Config = `{"webhook_secret":""}`
				require.NoError(t, db.Model(&gateway).Updates(map[string]any{"type": kind, "config": gateway.Config}).Error)
				body := fmt.Sprintf(`{"api_version":%q,"type":"checkout.session.completed","data":{"object":{"client_reference_id":%q,"payment_intent":"unsigned-fixture","payment_status":"paid","amount_total":700,"currency":"cny","metadata":{"user_id":"1"}}}}`, stripeapi.APIVersion, order.TradeNo)
				signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: []byte(body), Secret: ""})
				request := httptest.NewRequest("POST", "/callback", strings.NewReader(body))
				request.Header.Set("Stripe-Signature", signed.Header)
				response = runPaymentRequest(gateway, request)
				require.Equal(t, 503, response.Code)
			}
			requireQuotaPair(t, db, 1, 1000, 1000, 0)
		})
	}
}

func TestQuotaTransactionPaymentHistoricalClaim(t *testing.T) {
	for _, marker := range []int64{0, 1} {
		t.Run(fmt.Sprint(marker), func(t *testing.T) {
			db, gateway, oldOrder, client := paymentTransactionFixture(t, false)
			require.NoError(t, db.Model(&oldOrder).Updates(map[string]any{"status": model.OrderStatusSuccess, "gateway_no": "historical-transaction", "settled_at": marker}).Error)
			next := oldOrder
			next.Status = model.OrderStatusPending
			next.GatewayNo = ""
			next.SettledAt = 0
			next.ID = 0
			next.TradeNo = "new-order-historical-transaction"
			require.NoError(t, db.Create(&next).Error)
			require.NotEqual(t, "success", paymentCallbackFixture(gateway, next, client, "historical-transaction").Body.String())
			requireQuotaPair(t, db, 1, 1000, 1000, 0)
		})
	}
}
