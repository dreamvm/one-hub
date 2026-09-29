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
