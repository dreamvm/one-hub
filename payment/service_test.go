package payment_test

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"one-api/model"
	"one-api/payment"
)

func TestCallbackAcknowledgement(t *testing.T) {
	for _, test := range []struct {
		kind    string
		success bool
		status  int
		body    string
	}{
		{"epay", true, 200, "success"}, {"epay", false, 200, "fail"}, {"alipay", true, 200, "success"}, {"alipay", false, 200, "failure"}, {"wxpay", true, 204, ""}, {"wxpay", false, 503, `{"code":"FAIL","message":"callback not committed"}`}, {"stripe", true, 200, ""}, {"stripe", false, 503, ""},
	} {
		t.Run(test.kind+test.body, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			service := payment.PaymentService{Payment: &model.Payment{Type: test.kind}}
			service.RespondCallback(c, test.success)
			c.Writer.WriteHeaderNow()
			require.Equal(t, test.status, recorder.Code)
			require.Equal(t, test.body, recorder.Body.String())
		})
	}
}

func TestPaymentCurrencyAdmission(t *testing.T) {
	for _, kind := range []string{"epay", "stripe", "alipay", "wxpay"} {
		for _, currency := range []model.CurrencyType{model.CurrencyTypeCNY, model.CurrencyTypeUSD, "EUR"} {
			service := payment.PaymentService{Payment: &model.Payment{Type: kind, Currency: currency}}
			err := service.ValidatePay(7)
			if currency == model.CurrencyTypeCNY || currency == model.CurrencyTypeUSD && (kind == "epay" || kind == "stripe") {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			require.Error(t, service.ValidatePay(0))
			require.Error(t, service.ValidatePay(7.001))
		}
	}
}
