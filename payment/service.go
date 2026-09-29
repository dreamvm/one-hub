package payment

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"one-api/common/config"
	"one-api/common/logger"
	"one-api/model"
	"one-api/payment/types"

	"github.com/gin-gonic/gin"
)

type PaymentService struct {
	Payment *model.Payment
	gateway PaymentProcessor
}

type PayMoney struct {
	Amount   float64
	Currency model.CurrencyType
}

func NewPaymentService(uuid string) (*PaymentService, error) {
	payment, err := model.GetPaymentByUUID(uuid)
	if err != nil {
		return nil, errors.New("payment not found")
	}

	gateway, ok := Gateways[payment.Type]
	if !ok {
		return nil, errors.New("payment gateway not found")
	}

	return &PaymentService{
		Payment: payment,
		gateway: gateway,
	}, nil
}

func (s *PaymentService) CreatedPay() error {
	notifyURL := s.getNotifyURL()
	return s.gateway.CreatedPay(notifyURL, s.Payment)
}

func (s *PaymentService) ValidatePay(amount float64) error {
	if _, err := model.PaymentMinorFromAmount(amount); err != nil {
		return err
	}
	if s.Payment.Currency != model.CurrencyTypeCNY && s.Payment.Currency != model.CurrencyTypeUSD {
		return errors.New("unsupported payment currency")
	}
	if (s.Payment.Type == "alipay" || s.Payment.Type == "wxpay") && s.Payment.Currency != model.CurrencyTypeCNY {
		return errors.New("this payment gateway requires CNY")
	}
	return nil
}

func (s *PaymentService) Pay(tradeNo string, amount float64, user *model.User) (*types.PayRequest, error) {
	if err := s.ValidatePay(amount); err != nil {
		return nil, err
	}
	config := &types.PayConfig{
		Money:     amount,
		TradeNo:   tradeNo,
		NotifyURL: s.getNotifyURL(),
		ReturnURL: s.getReturnURL(),
		Currency:  s.Payment.Currency,
		User:      user,
	}
	payRequest, err := s.gateway.Pay(config, s.Payment.Config)
	if err != nil {
		return nil, err
	}

	return payRequest, nil
}

func (s *PaymentService) HandleCallback(c *gin.Context, gatewayConfig string) (*types.PayNotify, error) {
	payNotify, err := s.gateway.HandleCallback(c, gatewayConfig)
	if err != nil {
		logger.SysError(fmt.Sprintf("%s payment callback error: %v", s.gateway.Name(), err))

	}

	if err == nil && payNotify != nil && s.Payment.Type == "epay" {
		payNotify.Currency = s.Payment.Currency
	}
	return payNotify, err
}

func (s *PaymentService) getNotifyURL() string {
	notifyDomain := s.Payment.NotifyDomain
	if notifyDomain == "" {
		notifyDomain = config.ServerAddress
	}

	notifyDomain = strings.TrimSuffix(notifyDomain, "/")
	return fmt.Sprintf("%s/api/payment/notify/%s", notifyDomain, s.Payment.UUID)
}

func (s *PaymentService) getReturnURL() string {
	serverAdd := strings.TrimSuffix(config.ServerAddress, "/")
	return fmt.Sprintf("%s/panel/log", serverAdd)
}

// RespondCallback is called only after verification and durable settlement.
func (s *PaymentService) RespondCallback(c *gin.Context, success bool) {
	switch s.Payment.Type {
	case "epay", "alipay":
		body := "success"
		if !success {
			body = "fail"
			if s.Payment.Type == "alipay" {
				body = "failure"
			}
		}
		c.String(http.StatusOK, body)
	case "wxpay":
		if success {
			c.Status(http.StatusNoContent)
		} else {
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": "FAIL", "message": "callback not committed"})
		}
	default:
		if success {
			c.Status(http.StatusOK)
		} else {
			c.Status(http.StatusServiceUnavailable)
		}
	}
}
