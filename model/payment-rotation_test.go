package model_test

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"

	"one-api/payment"
	"one-api/payment/gateway/wxpay"
)

func TestQuotaTransactionPaymentWeChatRotation(t *testing.T) {
	db, gateway, order, _ := paymentTransactionFixture(t, false)
	merchant := "wechat-rotation-fixture"
	originalGateway := payment.Gateways["wxpay"]
	instance := &wxpay.WeChatPay{}
	payment.Gateways["wxpay"] = instance
	t.Cleanup(func() { instance.Close(); payment.Gateways["wxpay"] = originalGateway })

	oldTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = oldTransport
		downloader.MgrInstance().RemoveDownloader(context.Background(), merchant)
	})
	for revision := 0; revision < 2; revision++ {
		key, private, _ := paymentRSAFixture(t)
		apiKey := fmt.Sprintf("%032d", revision+1)
		serial := fmt.Sprintf("%X", 30000+revision)
		cert := &x509.Certificate{SerialNumber: big.NewInt(int64(30000 + revision)), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
		der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
		require.NoError(t, err)
		certificate := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
		downloadBody, err := json.Marshal(map[string]any{"data": []any{map[string]any{"serial_no": serial, "encrypt_certificate": encryptPaymentFixture(t, apiKey, certificate)}}})
		require.NoError(t, err)
		http.DefaultTransport = paymentRoundTripper(func(request *http.Request) (*http.Response, error) {
			if request.URL.Host != "api.mch.weixin.qq.com" || request.URL.Path != "/v3/certificates" || !strings.Contains(request.Header.Get("Authorization"), `serial_no="`+serial+`"`) {
				return nil, fmt.Errorf("obsolete or unexpected certificate request")
			}
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(string(downloadBody))), Request: request}, nil
		})
		if revision == 0 {
			client, err := core.NewClient(context.Background(), option.WithMerchantCredential(merchant, serial, key), option.WithoutValidator())
			require.NoError(t, err)
			require.NoError(t, downloader.MgrInstance().RegisterDownloaderWithClient(context.Background(), client, merchant, apiKey))
		}
		gateway.Type = "wxpay"
		conf, err := json.Marshal(wxpay.WeChatConfig{AppID: "rotation-app", MchID: merchant, MchPrivateKey: private, MchAPIv3Key: apiKey, MchCertificateSerialNumber: serial})
		require.NoError(t, err)
		gateway.Config = string(conf)
		require.NoError(t, db.Model(&gateway).Updates(map[string]any{"type": gateway.Type, "config": gateway.Config}).Error)
		if revision > 0 {
			order.ID = 0
			order.TradeNo = "rotation-second-order"
			require.NoError(t, order.Insert())
		}
		plain := fmt.Sprintf(`{"appid":"rotation-app","mchid":%q,"out_trade_no":%q,"transaction_id":%q,"trade_state":"SUCCESS","amount":{"total":700,"currency":"CNY"}}`, merchant, order.TradeNo, fmt.Sprintf("rotation-transaction-%d", revision))
		resource := encryptPaymentFixture(t, apiKey, plain)
		resource["original_type"] = "transaction"
		body, err := json.Marshal(map[string]any{"id": "rotation-event", "event_type": "TRANSACTION.SUCCESS", "resource_type": "encrypt-resource", "resource": resource})
		require.NoError(t, err)
		request := httptest.NewRequest("POST", "/callback", strings.NewReader(string(body)))
		stamp := strconv.FormatInt(time.Now().Unix(), 10)
		nonce := "rotation-notify"
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Wechatpay-Timestamp", stamp)
		request.Header.Set("Wechatpay-Nonce", nonce)
		request.Header.Set("Wechatpay-Serial", serial)
		request.Header.Set("Wechatpay-Signature", signPaymentFixture(t, key, stamp+"\n"+nonce+"\n"+string(body)+"\n"))
		require.Equal(t, 204, runPaymentRequest(gateway, request).Code, "new credential/certificate configuration must take effect without restart")
		requireQuotaPair(t, db, 1, 1000+(revision+1)*100, 1000, 0)
	}
}
