package model_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"one-api/common"
	"one-api/common/config"
	"one-api/controller"
	"one-api/model"
	"one-api/payment"
	"one-api/payment/types"
)

type orderAdmissionGateway struct {
	pay func(*types.PayConfig) (*types.PayRequest, error)
}

func (g *orderAdmissionGateway) Name() string                            { return "fixture" }
func (g *orderAdmissionGateway) CreatedPay(string, *model.Payment) error { return nil }
func (g *orderAdmissionGateway) HandleCallback(*gin.Context, string) (*types.PayNotify, error) {
	return nil, errors.New("unused fixture callback")
}
func (g *orderAdmissionGateway) Pay(c *types.PayConfig, _ string) (*types.PayRequest, error) {
	return g.pay(c)
}

func createOrderFixture(gateway model.Payment, amount int) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Set("id", 1)
	c.Request = httptest.NewRequest("POST", "/order", strings.NewReader(fmt.Sprintf(`{"uuid":%q,"amount":%d}`, gateway.UUID, amount)))
	c.Request.Header.Set("Content-Type", "application/json")
	controller.CreateOrder(c)
	return response
}
func installOrderGateway(t *testing.T, hook func(*types.PayConfig) (*types.PayRequest, error)) {
	t.Helper()
	original := payment.Gateways["epay"]
	payment.Gateways["epay"] = &orderAdmissionGateway{pay: hook}
	t.Cleanup(func() { payment.Gateways["epay"] = original })
	oldUnit, oldRate, oldMinimum, oldDiscount := config.QuotaPerUnit, config.PaymentUSDRate, config.PaymentMinAmount, common.RechargeDiscount
	config.QuotaPerUnit, config.PaymentUSDRate, config.PaymentMinAmount, common.RechargeDiscount = 500000, 7.3, 1, map[string]float64{}
	t.Cleanup(func() {
		config.QuotaPerUnit, config.PaymentUSDRate, config.PaymentMinAmount, common.RechargeDiscount = oldUnit, oldRate, oldMinimum, oldDiscount
	})
}

func TestQuotaTransactionPaymentOrderAdmission(t *testing.T) {
	for _, stage := range []string{"normal", "database failure", "quota overflow", "quota nonfinite", "zero discount", "nonfinite rate", "money overflow"} {
		t.Run(stage, func(t *testing.T) {
			db, gateway, _, _ := paymentTransactionFixture(t, false)
			calls := 0
			installOrderGateway(t, func(c *types.PayConfig) (*types.PayRequest, error) {
				calls++
				return &types.PayRequest{Type: 1, Data: types.PayRequestData{URL: "https://fixture.invalid/pay"}}, nil
			})
			amount := 1
			switch stage {
			case "database failure":
				require.NoError(t, db.Callback().Create().Before("gorm:create").Register("order_insert_failure", func(tx *gorm.DB) {
					if tx.Statement.Schema != nil && strings.HasSuffix(tx.Statement.Schema.Table, "_orders") {
						tx.AddError(errors.New("fixture order insert failed"))
					}
				}))
				t.Cleanup(func() { require.NoError(t, db.Callback().Create().Remove("order_insert_failure")) })
			case "quota overflow":
				config.QuotaPerUnit = math.MaxFloat64
			case "quota nonfinite":
				config.QuotaPerUnit = math.NaN()
			case "zero discount":
				common.RechargeDiscount["1"] = 0
			case "nonfinite rate":
				config.PaymentUSDRate = math.NaN()
			case "money overflow":
				amount = 100000000
			}
			response := createOrderFixture(gateway, amount)
			var result struct {
				Success bool `json:"success"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
			if stage == "normal" {
				require.True(t, result.Success)
				require.Equal(t, 1, calls)
			} else {
				require.False(t, result.Success)
				require.Zero(t, calls, "invalid or unpersisted orders must not reach external payment")
			}
		})
	}
}

func TestQuotaTransactionPaymentEarlyCallback(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		t.Run(fmt.Sprint(timeout), func(t *testing.T) {
			db, gateway, _, _ := paymentTransactionFixture(t, false)
			found := false
			var savedTrade string
			var callbackErr error
			installOrderGateway(t, func(c *types.PayConfig) (*types.PayRequest, error) {
				savedTrade = c.TradeNo
				order, err := model.GetOrderByTradeNo(c.TradeNo)
				if err != nil {
					return nil, err
				}
				found = true
				amount, err := model.PaymentMinorFromAmount(c.Money)
				if err != nil {
					return nil, err
				}
				callbackErr = model.SettleOrderPayment(gateway.ID, model.PaymentNotification{TradeNo: order.TradeNo, GatewayNo: "early-paid-fixture", AmountMinor: amount, Currency: c.Currency}, "127.0.0.1")
				if timeout {
					return nil, errors.New("ambiguous upstream timeout after payment")
				}
				return &types.PayRequest{Type: 1, Data: types.PayRequestData{URL: "https://fixture.invalid/pay"}}, nil
			})
			createOrderFixture(gateway, 1)
			require.True(t, found, "order exists before provider request can trigger callback")
			require.NoError(t, callbackErr)
			order, err := model.GetOrderByTradeNo(savedTrade)
			require.NoError(t, err)
			require.Equal(t, model.OrderStatusSuccess, order.Status)
			requireQuotaPair(t, db, 1, 501000, 1000, 0)
		})
	}
}

func TestQuotaTransactionPaymentPricingControls(t *testing.T) {
	for _, test := range []struct {
		name                                           string
		amount                                         int
		currency                                       model.CurrencyType
		discount, percent, fixed, paid, fee, reduction float64
	}{
		{"discount and percent", 10, model.CurrencyTypeCNY, 0.9, 0.02, 0, 67.01, 0.18, 7.45},
		{"fixed fee", 1, model.CurrencyTypeUSD, 1, 0, 0.1, 1.1, 0.1, -0.1},
		{"legacy rounding", 1, model.CurrencyTypeUSD, 2.675, 0, 0, 2.67, 0, -1.67},
		{"legacy percentage rounding", 1, model.CurrencyTypeUSD, 0.9, 0.05, 0, 0.95, 0.05, 0.1},
		{"large valid quota", 10000, model.CurrencyTypeCNY, 1, 0, 0, 73000, 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, gateway, _, _ := paymentTransactionFixture(t, false)
			var trade string
			installOrderGateway(t, func(c *types.PayConfig) (*types.PayRequest, error) {
				trade = c.TradeNo
				require.Equal(t, test.paid, c.Money)
				return &types.PayRequest{Type: 1, Data: types.PayRequestData{URL: "https://fixture.invalid/pay"}}, nil
			})
			common.RechargeDiscount[fmt.Sprint(test.amount)] = test.discount
			require.NoError(t, db.Model(&gateway).Updates(map[string]any{"currency": test.currency, "percent_fee": test.percent, "fixed_fee": test.fixed}).Error)
			if test.currency == model.CurrencyTypeUSD {
				config.PaymentUSDRate = math.NaN()
			}
			response := createOrderFixture(gateway, test.amount)
			var result struct {
				Success bool `json:"success"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
			require.True(t, result.Success)
			order, err := model.GetOrderByTradeNo(trade)
			require.NoError(t, err)
			require.Equal(t, test.paid, order.OrderAmount)
			require.Equal(t, test.fee, order.Fee)
			require.InDelta(t, test.reduction, order.Discount, 0.000001)
			require.Equal(t, test.amount*500000, order.Quota)
		})
	}
}

func TestQuotaTransactionPaymentUnknownCreationOutcome(t *testing.T) {
	for _, nilResponse := range []bool{false, true} {
		t.Run(fmt.Sprint(nilResponse), func(t *testing.T) {
			db, gateway, _, _ := paymentTransactionFixture(t, false)
			var trade string
			installOrderGateway(t, func(c *types.PayConfig) (*types.PayRequest, error) {
				trade = c.TradeNo
				if nilResponse {
					return nil, nil
				}
				return nil, errors.New("upstream timeout")
			})
			response := createOrderFixture(gateway, 1)
			var result struct {
				Success bool `json:"success"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
			require.False(t, result.Success)
			order, err := model.GetOrderByTradeNo(trade)
			require.NoError(t, err)
			require.Equal(t, model.OrderStatusPending, order.Status)
			amount, err := model.PaymentMinorFromAmount(order.OrderAmount)
			require.NoError(t, err)
			require.NoError(t, model.SettleOrderPayment(gateway.ID, model.PaymentNotification{TradeNo: trade, GatewayNo: "late-after-timeout", AmountMinor: amount, Currency: order.OrderCurrency}, "127.0.0.1"))
			requireQuotaPair(t, db, 1, 501000, 1000, 0)
		})
	}
}
