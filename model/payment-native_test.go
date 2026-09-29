package model_test

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"one-api/model"
	"one-api/payment/gateway/wxpay"
	"one-api/payment/types"
)

func TestPaymentWeChatMinorAmount(t *testing.T) {
	key, private, _ := paymentRSAFixture(t)
	serial := "4567"
	apiKey := fmt.Sprintf("%032d", 73)
	cert := &x509.Certificate{SerialNumber: big.NewInt(0x4567), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	require.NoError(t, err)
	certificate := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	certResponse, err := json.Marshal(map[string]any{"data": []any{map[string]any{"serial_no": serial, "encrypt_certificate": encryptPaymentFixture(t, apiKey, certificate)}}})
	require.NoError(t, err)
	oldTransport := http.DefaultTransport
	gateway := &wxpay.WeChatPay{}
	t.Cleanup(func() { gateway.Close(); http.DefaultTransport = oldTransport })
	var observed int64
	http.DefaultTransport = paymentRoundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "api.mch.weixin.qq.com" {
			return nil, fmt.Errorf("unexpected fixture host")
		}
		responseBody := string(certResponse)
		switch request.URL.Path {
		case "/v3/certificates":
		case "/v3/pay/transactions/native":
			var sent struct {
				Amount struct {
					Total int64 `json:"total"`
				} `json:"amount"`
			}
			if err := json.NewDecoder(request.Body).Decode(&sent); err != nil {
				return nil, err
			}
			observed = sent.Amount.Total
			responseBody = `{"code_url":"https://fixture.invalid/native"}`
		default:
			return nil, fmt.Errorf("unexpected fixture path")
		}
		stamp := strconv.FormatInt(time.Now().Unix(), 10)
		nonce := "native-response"
		headers := http.Header{"Content-Type": {"application/json"}}
		headers.Set("Wechatpay-Timestamp", stamp)
		headers.Set("Wechatpay-Nonce", nonce)
		headers.Set("Wechatpay-Serial", serial)
		headers.Set("Wechatpay-Signature", signPaymentFixture(t, key, stamp+"\n"+nonce+"\n"+responseBody+"\n"))
		return &http.Response{StatusCode: 200, Header: headers, Body: io.NopCloser(strings.NewReader(responseBody)), Request: request}, nil
	})
	config, err := json.Marshal(wxpay.WeChatConfig{AppID: "native-app", MchID: "native-merchant", MchPrivateKey: private, MchCertificateSerialNumber: serial, MchAPIv3Key: apiKey})
	require.NoError(t, err)
	for _, money := range []float64{4, 4.1} {
		t.Run(fmt.Sprint(money), func(t *testing.T) {
			result, err := gateway.Pay(&types.PayConfig{Money: money, Currency: model.CurrencyTypeCNY, TradeNo: "native-order", NotifyURL: "https://fixture.invalid/notify"}, string(config))
			require.NoError(t, err)
			require.Equal(t, "https://fixture.invalid/native", result.Data.URL)
			expected := int64(400)
			if money == 4.1 {
				expected = 410
			}
			require.Equal(t, expected, observed)
		})
	}
}
