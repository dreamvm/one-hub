package wxpay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"one-api/model"
	"one-api/payment/types"

	"github.com/gin-gonic/gin"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
)

type WeChatPay struct {
	mu       sync.Mutex
	runtimes map[string]*wechatRuntime
}

type WeChatConfig struct {
	AppID                      string  `json:"app_id"`                        //应用ID
	MchID                      string  `json:"mch_id"`                        //商户号
	MchCertificateSerialNumber string  `json:"mch_certificate_serial_number"` //商户证书序列号
	MchAPIv3Key                string  `json:"mch_apiv3_key"`                 //商户APIv3密钥
	MchPrivateKey              string  `json:"mch_private_key"`               //商户私钥
	NotifyURL                  string  `json:"notify_url"`
	PayType                    PayType `json:"pay_type"`
}

func (w *WeChatPay) Name() string {
	return "微信支付"
}

func (w *WeChatPay) Pay(config *types.PayConfig, gatewayConfig string) (*types.PayRequest, error) {
	wechatConfig, err := getWeChatConfig(gatewayConfig)
	if err != nil {
		return nil, err
	}

	runtime, err := w.runtime(context.Background(), wechatConfig)
	if err != nil {
		return nil, err
	}

	switch wechatConfig.PayType {
	case Native:
		return w.handleNativePay(runtime.client, config, wechatConfig)
	default:
		return w.handleNativePay(runtime.client, config, wechatConfig)
	}
}

func (w *WeChatPay) HandleCallback(c *gin.Context, gatewayConfig string) (*types.PayNotify, error) {

	wxpayConfig, err := getWeChatConfig(gatewayConfig)
	if err != nil {
		// 接收失败，返回4XX或5XX状态码以及应答报文
		return nil, fmt.Errorf("WeChat params failed: %v", err)
	}
	runtime, err := w.runtime(c.Request.Context(), wxpayConfig)
	if err != nil {
		return nil, err
	}
	certificateVisitor := runtime.manager.GetCertificateVisitor(wxpayConfig.MchID)
	handler := notify.NewNotifyHandler(wxpayConfig.MchAPIv3Key, verifiers.NewSHA256WithRSAVerifier(certificateVisitor))
	transaction := new(payments.Transaction)
	notifyReq, err := handler.ParseNotifyRequest(c.Request.Context(), c.Request, transaction)
	// 如果验签未通过，或者解密失败
	if err != nil {
		// 接收失败，返回4XX或5XX状态码以及应答报文
		return nil, fmt.Errorf("WeChat Signature verification failed: %v", err)
	}
	if notifyReq.EventType != "TRANSACTION.SUCCESS" {
		return nil, fmt.Errorf("WeChat Transaction failed: %v", notifyReq.EventType)
	}
	if transaction.TradeState == nil || transaction.OutTradeNo == nil || transaction.TransactionId == nil || *transaction.TradeState != "SUCCESS" {
		return nil, errors.New("incomplete or unsuccessful payment notification")
	}

	if transaction.Appid == nil || *transaction.Appid != wxpayConfig.AppID || wxpayConfig.AppID == "" || transaction.Mchid == nil || *transaction.Mchid != wxpayConfig.MchID || wxpayConfig.MchID == "" || transaction.Amount == nil || transaction.Amount.Total == nil || *transaction.Amount.Total <= 0 || transaction.Amount.Currency == nil {
		return nil, errors.New("incomplete or mismatched payment facts")
	}
	payNotify := &types.PayNotify{
		TradeNo:     *transaction.OutTradeNo,
		GatewayNo:   *transaction.TransactionId,
		AmountMinor: *transaction.Amount.Total,
		Currency:    model.CurrencyType(*transaction.Amount.Currency),
	}
	return payNotify, nil

}

func getWeChatConfig(gatewayConfig string) (*WeChatConfig, error) {
	var wechatConfig WeChatConfig
	if err := json.Unmarshal([]byte(gatewayConfig), &wechatConfig); err != nil {
		return nil, errors.New("config error")
	}

	return &wechatConfig, nil
}

func (w *WeChatPay) CreatedPay(_ string, _ *model.Payment) error {
	return nil
}
