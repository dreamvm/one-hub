package wxpay

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"

	sysconfig "one-api/common/config"
	"one-api/model"
	"one-api/payment/types"
)

// handleNativePay 处理微信NATIVE支付请求
func (w *WeChatPay) handleNativePay(client *core.Client, config *types.PayConfig, wechatConfig *WeChatConfig) (*types.PayRequest, error) {
	minor, err := model.PaymentMinorFromAmount(config.Money)
	if err != nil {
		return nil, err
	}
	totalAmount := strconv.FormatFloat(config.Money, 'f', 0, 64)
	req := native.PrepayRequest{
		Appid:       core.String(wechatConfig.AppID),
		Mchid:       core.String(wechatConfig.MchID),
		Description: core.String(sysconfig.SystemName + "-Token充值:" + totalAmount),
		OutTradeNo:  core.String(config.TradeNo),
		NotifyUrl:   core.String(config.NotifyURL),
		Amount: &native.Amount{
			Total: core.Int64(minor), // 转换为分
		},
	}
	nService := native.NativeApiService{Client: client}
	resp, result, err := nService.Prepay(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("wechat native pay failed: %s", err.Error())
	}
	if result.Response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wechat native pay failed: %s", result.Response.Status)
	}

	payRequest := &types.PayRequest{
		Type: 2,
		Data: types.PayRequestData{
			URL:    *resp.CodeUrl,
			Method: http.MethodGet,
		},
	}
	return payRequest, nil
}
