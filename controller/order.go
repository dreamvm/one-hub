package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"one-api/common"
	"one-api/common/config"
	"one-api/common/logger"
	"one-api/common/utils"
	"one-api/model"
	"one-api/payment"
	"one-api/payment/types"

	"github.com/gin-gonic/gin"
)

type OrderRequest struct {
	UUID   string `json:"uuid" binding:"required"`
	Amount int    `json:"amount" binding:"required"`
}

type OrderResponse struct {
	TradeNo string `json:"trade_no"`
	*types.PayRequest
}

// CreateOrder
func CreateOrder(c *gin.Context) {
	var orderReq OrderRequest
	if err := c.ShouldBindJSON(&orderReq); err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invalid request"))

		return
	}

	if orderReq.Amount <= 0 || orderReq.Amount < config.PaymentMinAmount {
		common.APIRespondWithError(c, http.StatusOK, fmt.Errorf("金额必须大于等于 %d", config.PaymentMinAmount))

		return
	}

	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("用户不存在"))
		return
	}

	// 关闭用户未完成的订单
	go model.CloseUnfinishedOrder()

	paymentService, err := payment.NewPaymentService(orderReq.UUID)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	// 获取手续费和支付金额
	discount, fee, payMoney := calculateOrderAmount(paymentService.Payment, orderReq.Amount)
	// 开始支付
	tradeNo := utils.GenerateTradeNo()
	payRequest, err := paymentService.Pay(tradeNo, payMoney, user)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("创建支付失败，请稍后再试"))
		return
	}

	// 创建订单
	order := &model.Order{
		UserId:        userId,
		GatewayId:     paymentService.Payment.ID,
		TradeNo:       tradeNo,
		Amount:        orderReq.Amount,
		OrderAmount:   payMoney,
		OrderCurrency: paymentService.Payment.Currency,
		Fee:           fee,
		Discount:      discount,
		Status:        model.OrderStatusPending,
		Quota:         orderReq.Amount * int(config.QuotaPerUnit),
	}

	err = order.Insert()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("创建订单失败，请稍后再试"))
		return
	}

	orderResp := &OrderResponse{
		TradeNo:    tradeNo,
		PayRequest: payRequest,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    orderResp,
	})
}

func PaymentCallback(c *gin.Context) {
	paymentService, err := payment.NewPaymentService(c.Param("uuid"))
	if err != nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	notification, err := paymentService.HandleCallback(c, paymentService.Payment.Config)
	if err != nil {
		paymentService.RespondCallback(c, false)
		return
	}
	// A verified event unrelated to fulfillment is acknowledged without a credit.
	if notification == nil {
		paymentService.RespondCallback(c, true)
		return
	}
	err = model.SettleOrderPayment(paymentService.Payment.ID, *notification, c.ClientIP())
	if err != nil {
		logger.SysError("payment callback settlement failed; provider retry required")
	}
	paymentService.RespondCallback(c, err == nil)
}

func CheckOrderStatus(c *gin.Context) {
	tradeNo := c.Query("trade_no")
	userId := c.GetInt("id")
	success := false

	if tradeNo != "" {
		order, err := model.GetUserOrder(userId, tradeNo)
		if err == nil {
			if order.Status == model.OrderStatusSuccess {
				success = true
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": success,
		"message": "",
	})
}

// discountMoney优惠金额 fee手续费，payMoney实付金额
func calculateOrderAmount(payment *model.Payment, amount int) (discountMoney, fee, payMoney float64) {
	// 获取折扣
	discount := common.GetRechargeDiscount(strconv.Itoa(amount))
	newMoney := float64(amount) * discount // 折后价值
	oldTotal := float64(amount)            //原价值
	if payment.PercentFee > 0 {
		//手续费=（原始价值*折扣*手续费率）
		fee = utils.Decimal(newMoney*payment.PercentFee, 2) //折后手续
		oldTotal = utils.Decimal(oldTotal*(1+payment.PercentFee), 2)
	} else if payment.FixedFee > 0 {
		//固定费率不计算折扣
		fee = payment.FixedFee
	}

	//实际费用=（折后价+折后手续费）*汇率
	total := utils.Decimal(newMoney+fee, 2)
	if payment.Currency == model.CurrencyTypeUSD {
		payMoney = total
	} else {
		oldTotal = utils.Decimal(oldTotal*config.PaymentUSDRate, 2)
		payMoney = utils.Decimal(total*config.PaymentUSDRate, 2)
	}
	discountMoney = oldTotal - payMoney //折扣金额 = 原价值-实际支付价值
	return
}

func GetOrderList(c *gin.Context) {
	var params model.SearchOrderParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	payments, err := model.GetOrderList(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    payments,
	})
}
