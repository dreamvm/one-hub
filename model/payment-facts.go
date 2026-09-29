package model

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

// PaymentNotification contains only facts obtained from the authenticated
// provider payload. Currency is supplied from configuration only for Epay,
// whose callback protocol has no currency field.
type PaymentNotification struct {
	TradeNo     string       `json:"trade_no"`
	GatewayNo   string       `json:"gateway_no"`
	AmountMinor int64        `json:"amount_minor"`
	Currency    CurrencyType `json:"currency"`
	UserID      int          `json:"user_id,omitempty"`
}

// OrderPaymentClaim binds one gateway transaction to one local order. The ID is
// a hash of gateway ID plus the complete transaction ID, not an amount or user.
type OrderPaymentClaim struct {
	ID      string `gorm:"type:char(64);primaryKey"`
	OrderID int    `gorm:"uniqueIndex;not null"`
}

func ParsePaymentMinor(value string) (int64, error) {
	parts := strings.Split(value, ".")
	if len(parts) > 2 || len(parts[0]) == 0 || len(parts[0]) > 8 {
		return 0, errors.New("invalid payment amount")
	}
	fraction := "00"
	if len(parts) == 2 {
		if len(parts[1]) < 1 || len(parts[1]) > 2 {
			return 0, errors.New("invalid payment precision")
		}
		fraction = parts[1]
		if len(fraction) == 1 {
			fraction += "0"
		}
	}
	for _, c := range parts[0] + fraction {
		if c < '0' || c > '9' {
			return 0, errors.New("invalid payment amount")
		}
	}
	minor, err := strconv.ParseInt(parts[0]+fraction, 10, 64)
	if err != nil || minor <= 0 || minor > 9999999999 {
		return 0, errors.New("invalid payment amount")
	}
	return minor, nil
}

func PaymentMinorFromAmount(amount float64) (int64, error) {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 || amount > 99999999.99 {
		return 0, errors.New("invalid stored payment amount")
	}
	minor := int64(math.Round(amount * 100))
	if amount != float64(minor)/100 {
		return 0, errors.New("invalid stored payment precision")
	}
	return minor, nil
}
