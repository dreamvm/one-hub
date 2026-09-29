package epay

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"one-api/model"
	"one-api/payment/types"

	"github.com/gin-gonic/gin"
)

type Epay struct{}

type EpayConfig struct {
	PayType PayType `json:"pay_type"`
	Client
}

func (e *Epay) Name() string {
	return "易支付"
}

func (e *Epay) Pay(config *types.PayConfig, gatewayConfig string) (*types.PayRequest, error) {
	epayConfig, err := getEpayConfig(gatewayConfig)
	if err != nil {
		return nil, err
	}

	payArgs := &PayArgs{
		Type:       epayConfig.PayType,
		OutTradeNo: config.TradeNo,
		NotifyUrl:  config.NotifyURL,
		ReturnUrl:  config.ReturnURL,
		Name:       config.TradeNo,
		Money:      strconv.FormatFloat(config.Money, 'f', 2, 64),
	}

	formPayURL, formPayArgs, err := epayConfig.FormPay(payArgs)
	if err != nil {
		return nil, err
	}

	payRequest := &types.PayRequest{
		Type: 1,
		Data: types.PayRequestData{
			URL:    formPayURL,
			Params: &formPayArgs,
			Method: http.MethodPost,
		},
	}

	return payRequest, nil
}

func (e *Epay) HandleCallback(c *gin.Context, gatewayConfig string) (*types.PayNotify, error) {
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		return nil, errors.New("invalid callback query")
	}
	queryMap := make(map[string]string, len(values))
	for key, value := range values {
		if len(value) != 1 {
			return nil, errors.New("ambiguous callback parameter")
		}
		queryMap[key] = value[0]
	}

	epayConfig, err := getEpayConfig(gatewayConfig)
	if err != nil {
		return nil, fmt.Errorf("tradeNo: %s, PaymentNo: %s,  err: %v", queryMap["out_trade_no"], queryMap["trade_no"], err)
	}

	paymentResult, success := epayConfig.Verify(queryMap)
	if paymentResult != nil && success {
		if queryMap["pid"] != epayConfig.PartnerID || epayConfig.PartnerID == "" || epayConfig.PayType != "" && paymentResult.Type != epayConfig.PayType {
			return nil, errors.New("payment merchant or method mismatch")
		}
		amount, err := model.ParsePaymentMinor(paymentResult.Money)
		if err != nil {
			return nil, err
		}
		payNotify := &types.PayNotify{
			TradeNo:     paymentResult.OutTradeNo,
			GatewayNo:   paymentResult.TradeNo,
			AmountMinor: amount,
		}
		return payNotify, nil
	}

	return nil, fmt.Errorf("tradeNo: %s, PaymentNo: %s,  Verify Sign failed", queryMap["out_trade_no"], queryMap["trade_no"])
}

func getEpayConfig(gatewayConfig string) (*EpayConfig, error) {
	var epayConfig EpayConfig
	if err := json.Unmarshal([]byte(gatewayConfig), &epayConfig); err != nil {
		return nil, errors.New("config error")
	}

	if epayConfig.Key == "" || epayConfig.PartnerID == "" {
		return nil, errors.New("missing payment signing configuration")
	}
	return &epayConfig, nil
}
func (e *Epay) CreatedPay(_ string, _ *model.Payment) error {
	return nil
}
