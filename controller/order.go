package controller

import (
	"errors"
	"fmt"
	"net/http"

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

	paymentService, err := payment.NewPaymentService(orderReq.UUID)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	discount, fee, payMoney, err := calculateOrderAmount(paymentService.Payment, orderReq.Amount)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	quota, err := calculateOrderQuota(orderReq.Amount)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if err := paymentService.ValidatePay(payMoney); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if err := model.CloseUnfinishedOrder(); err != nil {
		logger.SysError("failed to close expired payment orders")
	}
	tradeNo := utils.GenerateTradeNo()
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
		Quota:         quota,
	}

	err = order.Insert()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("创建订单失败，请稍后再试"))
		return
	}

	// A timeout does not establish that the provider failed to create/settle the
	// payment. Keep the durable pending order for a verified callback/reconciliation.
	payRequest, err := paymentService.Pay(tradeNo, payMoney, user)
	if err != nil || payRequest == nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("创建支付未确认，请稍后核对订单"))
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
