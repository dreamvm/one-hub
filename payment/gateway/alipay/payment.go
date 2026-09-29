package alipay

import (
	"encoding/json"
	"errors"
	"fmt"

	"one-api/model"
	"one-api/payment/types"

	"github.com/gin-gonic/gin"
	"github.com/smartwalle/alipay/v3"
)

type Alipay struct{}

type AlipayConfig struct {
	AppID      string  `json:"app_id"`
	PrivateKey string  `json:"private_key"`
	PublicKey  string  `json:"public_key"`
	PayType    PayType `json:"pay_type"`
}

const isProduction bool = true

func (a *Alipay) Name() string {
	return "支付宝"
}

func newClient(config *AlipayConfig) (*alipay.Client, error) {
	client, err := alipay.New(config.AppID, config.PrivateKey, isProduction)
	if err != nil {
		return nil, err
	}
	if err = client.LoadAliPayPublicKey(config.PublicKey); err != nil {
		return nil, err
	}
	return client, nil
}

func (a *Alipay) Pay(config *types.PayConfig, gatewayConfig string) (*types.PayRequest, error) {
	alipayConfig, err := getAlipayConfig(gatewayConfig)
	if err != nil {
		return nil, err
	}

	client, err := newClient(alipayConfig)
	if err != nil {
		return nil, err
	}

	switch alipayConfig.PayType {
	case PagePay:
		return a.handlePagePay(client, config)
	case WapPay:
		return a.handleWapPay(client, config)
	default:
		return a.handleTradePreCreate(client, config)
	}
}

func (a *Alipay) HandleCallback(c *gin.Context, gatewayConfig string) (*types.PayNotify, error) {
	config, err := getAlipayConfig(gatewayConfig)
	if err != nil {
		return nil, err
	}
	client, err := newClient(config)
	if err != nil {
		return nil, err
	}
	if err := c.Request.ParseForm(); err != nil {
		return nil, err
	}
	params := c.Request.Form
	for _, values := range params {
		if len(values) != 1 {
			return nil, errors.New("ambiguous callback parameter")
		}
	}
	//解析通知内容
	noti, err := client.DecodeNotification(params)
	if err != nil {
		return nil, fmt.Errorf("Alipay Error decoding notification: %v", err)
	}

	if noti.TradeStatus == alipay.TradeStatusSuccess || noti.TradeStatus == alipay.TradeStatusFinished {
		if config.AppID == "" || noti.AppId != config.AppID {
			return nil, errors.New("payment application mismatch")
		}
		amount, err := model.ParsePaymentMinor(noti.TotalAmount)
		if err != nil {
			return nil, err
		}
		payNotify := &types.PayNotify{
			TradeNo:     noti.OutTradeNo,
			GatewayNo:   noti.TradeNo,
			AmountMinor: amount,
			Currency:    model.CurrencyTypeCNY,
		}
		return payNotify, nil
	}
	return nil, fmt.Errorf("trade status not success")
}

func getAlipayConfig(gatewayConfig string) (*AlipayConfig, error) {
	var alipayConfig AlipayConfig
	if err := json.Unmarshal([]byte(gatewayConfig), &alipayConfig); err != nil {
		return nil, errors.New("config error")
	}

	return &alipayConfig, nil
}

func (a *Alipay) CreatedPay(_ string, _ *model.Payment) error {
	return nil
}
