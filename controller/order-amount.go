package controller

import (
	"errors"
	"math"
	"strconv"

	"github.com/shopspring/decimal"

	"one-api/common"
	"one-api/common/config"
	"one-api/common/utils"
	"one-api/model"
)

func finiteOrderValue(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func calculateOrderQuota(amount int) (int, error) {
	if amount <= 0 || !finiteOrderValue(config.QuotaPerUnit) || config.QuotaPerUnit <= 0 {
		return 0, errors.New("无效的充值额度配置")
	}
	quota := decimal.NewFromInt(int64(amount)).Mul(decimal.NewFromFloat(config.QuotaPerUnit))
	if !quota.IsInteger() || quota.LessThanOrEqual(decimal.Zero) || quota.GreaterThan(decimal.NewFromInt(int64(int(^uint(0)>>1)))) {
		return 0, errors.New("充值额度超出有效范围")
	}
	return int(quota.IntPart()), nil
}

func calculateOrderAmount(payment *model.Payment, amount int) (discountMoney, fee, payMoney float64, err error) {
	fail := func() (float64, float64, float64, error) {
		return 0, 0, 0, errors.New("无效的支付金额或费率配置")
	}
	discount := common.GetRechargeDiscount(strconv.Itoa(amount))
	if amount <= 0 || !finiteOrderValue(discount) || discount <= 0 || !finiteOrderValue(payment.PercentFee) || payment.PercentFee < 0 || !finiteOrderValue(payment.FixedFee) || payment.FixedFee < 0 {
		return fail()
	}
	if payment.Currency != model.CurrencyTypeUSD && payment.Currency != model.CurrencyTypeCNY {
		return fail()
	}
	// Preserve the existing pricing and UI rounding contract; validate before
	// conversion or any external payment side effect.
	net := float64(amount) * discount
	oldTotal := float64(amount)
	if payment.PercentFee > 0 {
		fee = utils.Decimal(net*payment.PercentFee, 2)
		oldTotal = utils.Decimal(oldTotal*(1+payment.PercentFee), 2)
	} else if payment.FixedFee > 0 {
		if _, err := model.PaymentMinorFromAmount(payment.FixedFee); err != nil {
			return fail()
		}
		fee = payment.FixedFee
	}
	payMoney = utils.Decimal(net+fee, 2)
	if payment.Currency == model.CurrencyTypeCNY {
		if !finiteOrderValue(config.PaymentUSDRate) || config.PaymentUSDRate <= 0 {
			return fail()
		}
		oldTotal = utils.Decimal(oldTotal*config.PaymentUSDRate, 2)
		payMoney = utils.Decimal(payMoney*config.PaymentUSDRate, 2)
	}
	discountMoney = oldTotal - payMoney
	for _, value := range []float64{net, oldTotal, fee, payMoney, discountMoney} {
		if !finiteOrderValue(value) || math.Abs(value) > 99999999.99 {
			return fail()
		}
	}
	if _, err := model.PaymentMinorFromAmount(payMoney); err != nil {
		return fail()
	}
	return discountMoney, fee, payMoney, nil
}
