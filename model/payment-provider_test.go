package model_test

import (
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"

	"one-api/controller"
	"one-api/model"
	"one-api/payment"
	"one-api/payment/gateway/alipay"
	"one-api/payment/gateway/wxpay"
)

func paymentRSAFixture(t *testing.T) (*rsa.PrivateKey, string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	private := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}))
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	public := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
	return key, private, public
}
func signPaymentFixture(t *testing.T, key *rsa.PrivateKey, text string) string {
	t.Helper()
	hash := sha256.Sum256([]byte(text))
	signed, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(signed)
}
func runPaymentRequest(gateway model.Payment, request *http.Request) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Params = gin.Params{{Key: "uuid", Value: gateway.UUID}}
	c.Request = request
	controller.PaymentCallback(c)
	c.Writer.WriteHeaderNow()
	return response
}

func TestQuotaTransactionPaymentAlipayFacts(t *testing.T) {
	key, private, public := paymentRSAFixture(t)
	for _, state := range []string{"normal after restart", "coupon", "wrong app", "underpayment", "bad signature", "duplicate parameter", "finished"} {
		t.Run(state, func(t *testing.T) {
			db, gateway, order, _ := paymentTransactionFixture(t, false)
			gateway.Type = "alipay"
			conf, err := json.Marshal(alipay.AlipayConfig{AppID: "fixture-application", PrivateKey: private, PublicKey: public})
			require.NoError(t, err)
			gateway.Config = string(conf)
			require.NoError(t, db.Model(&gateway).Updates(map[string]any{"type": gateway.Type, "config": gateway.Config}).Error)
			fields := url.Values{"app_id": {"fixture-application"}, "out_trade_no": {order.TradeNo}, "trade_no": {"alipay-fixture-transaction"}, "trade_status": {"TRADE_SUCCESS"}, "total_amount": {"7.00"}, "buyer_pay_amount": {"7.00"}, "sign_type": {"RSA2"}}
			switch state {
			case "coupon":
				fields.Set("buyer_pay_amount", "5.00")
			case "wrong app":
				fields.Set("app_id", "another-application")
			case "underpayment":
				fields.Set("total_amount", "0.01")
			case "finished":
				fields.Set("trade_status", "TRADE_FINISHED")
			}
			pairs := []string{}
			for k, v := range fields {
				if k != "sign_type" {
					pairs = append(pairs, k+"="+v[0])
				}
			}
			sort.Strings(pairs)
			fields.Set("sign", signPaymentFixture(t, key, strings.Join(pairs, "&")))
			if state == "bad signature" {
				fields.Set("sign", "invalid")
			}
			if state == "duplicate parameter" {
				fields.Add("total_amount", "0.01")
			}
			request := httptest.NewRequest("POST", "/callback", strings.NewReader(fields.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response := runPaymentRequest(gateway, request)
			if state == "normal after restart" || state == "coupon" || state == "finished" {
				require.Equal(t, "success", response.Body.String())
				requireQuotaPair(t, db, 1, 1100, 1000, 0)
			} else {
				require.Equal(t, "failure", response.Body.String())
				requireQuotaPair(t, db, 1, 1000, 1000, 0)
			}
		})
	}
}

type paymentRoundTripper func(*http.Request) (*http.Response, error)

func (f paymentRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func encryptPaymentFixture(t *testing.T, key, plain string) map[string]string {
	t.Helper()
	block, err := aes.NewCipher([]byte(key))
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)
	nonce := "fixtureNonce"
	associated := "transaction"
	return map[string]string{"algorithm": "AEAD_AES_256_GCM", "nonce": nonce, "associated_data": associated, "ciphertext": base64.StdEncoding.EncodeToString(gcm.Seal(nil, []byte(nonce), []byte(plain), []byte(associated)))}
}
func TestQuotaTransactionPaymentWeChatFacts(t *testing.T) {
	key, private, _ := paymentRSAFixture(t)
	material := make([]byte, 16)
	_, randomErr := rand.Read(material)
	require.NoError(t, randomErr)
	apiKey := hex.EncodeToString(material)
	cert := &x509.Certificate{SerialNumber: big.NewInt(12345), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	certDER, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	require.NoError(t, err)
	certificate := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	serial := "3039"
	merchant := "wechat-fixture-merchant"
	downloadBody, err := json.Marshal(map[string]any{"data": []any{map[string]any{"serial_no": serial, "encrypt_certificate": encryptPaymentFixture(t, apiKey, certificate)}}})
	require.NoError(t, err)
	// Only certificate download is mocked; notification RSA verification and GCM
	// decryption use the production SDK against generated local cryptographic data.
	mockHTTP := &http.Client{Transport: paymentRoundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "api.mch.weixin.qq.com" || request.URL.Path != "/v3/certificates" {
			return nil, fmt.Errorf("unexpected fixture request")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(string(downloadBody))), Request: request}, nil
	})}
	oldTransport := http.DefaultTransport
	http.DefaultTransport = mockHTTP.Transport
	originalGateway := payment.Gateways["wxpay"]
	instance := &wxpay.WeChatPay{}
	payment.Gateways["wxpay"] = instance
	t.Cleanup(func() {
		instance.Close()
		payment.Gateways["wxpay"] = originalGateway
		http.DefaultTransport = oldTransport
	})
	client, err := core.NewClient(context.Background(), option.WithMerchantCredential(merchant, serial, key), option.WithoutValidator(), option.WithHTTPClient(mockHTTP))
	require.NoError(t, err)
	require.NoError(t, downloader.MgrInstance().RegisterDownloaderWithClient(context.Background(), client, merchant, apiKey))
	t.Cleanup(func() { downloader.MgrInstance().RemoveDownloader(context.Background(), merchant) })
	for _, state := range []string{"normal", "coupon", "wrong app", "wrong merchant", "underpayment", "currency", "missing amount", "bad signature", "bad ciphertext"} {
		t.Run(state, func(t *testing.T) {
			db, gateway, order, _ := paymentTransactionFixture(t, false)
			gateway.Type = "wxpay"
			conf, err := json.Marshal(wxpay.WeChatConfig{AppID: "wechat-fixture-app", MchID: merchant, MchAPIv3Key: apiKey, MchPrivateKey: private, MchCertificateSerialNumber: serial})
			require.NoError(t, err)
			gateway.Config = string(conf)
			require.NoError(t, db.Model(&gateway).Updates(map[string]any{"type": gateway.Type, "config": gateway.Config}).Error)
			amount := map[string]any{"total": 700, "payer_total": 700, "currency": "CNY"}
			transaction := map[string]any{"appid": "wechat-fixture-app", "mchid": merchant, "out_trade_no": order.TradeNo, "transaction_id": "wechat-fixture-transaction", "trade_state": "SUCCESS", "amount": amount}
			switch state {
			case "coupon":
				amount["payer_total"] = 500
			case "wrong app":
				transaction["appid"] = "another-app"
			case "wrong merchant":
				transaction["mchid"] = "another-merchant"
			case "underpayment":
				amount["total"] = 1
			case "currency":
				amount["currency"] = "USD"
			case "missing amount":
				delete(transaction, "amount")
			}
			plaintext, err := json.Marshal(transaction)
			require.NoError(t, err)
			resource := encryptPaymentFixture(t, apiKey, string(plaintext))
			resource["original_type"] = "transaction"
			if state == "bad ciphertext" {
				resource["ciphertext"] = "invalid"
			}
			body, err := json.Marshal(map[string]any{"id": "wechat-event-fixture", "event_type": "TRANSACTION.SUCCESS", "resource_type": "encrypt-resource", "resource": resource})
			require.NoError(t, err)
			request := httptest.NewRequest("POST", "/callback", strings.NewReader(string(body)))
			stamp := strconv.FormatInt(time.Now().Unix(), 10)
			nonce := "notify-fixture"
			request.Header.Set("Wechatpay-Timestamp", stamp)
			request.Header.Set("Wechatpay-Nonce", nonce)
			request.Header.Set("Wechatpay-Serial", serial)
			request.Header.Set("Wechatpay-Signature", signPaymentFixture(t, key, stamp+"\n"+nonce+"\n"+string(body)+"\n"))
			request.Header.Set("Content-Type", "application/json")
			if state == "bad signature" {
				request.Header.Set("Wechatpay-Signature", "invalid")
			}
			response := runPaymentRequest(gateway, request)
			if state == "normal" || state == "coupon" {
				require.Equal(t, 204, response.Code)
				requireQuotaPair(t, db, 1, 1100, 1000, 0)
			} else {
				require.Equal(t, 503, response.Code)
				requireQuotaPair(t, db, 1, 1000, 1000, 0)
			}
		})
	}
}
