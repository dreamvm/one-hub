package relay_util

import (
	"context"
	"errors"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"one-api/common"
	"one-api/common/config"
	"one-api/common/logger"
	"one-api/model"
	"one-api/types"
)

type Quota struct {
	modelName        string
	promptTokens     int
	price            model.Price
	groupName        string
	isBackupGroup    bool // 新增字段记录是否使用备用分组
	backupGroupName  string
	groupRatio       float64
	inputRatio       float64
	outputRatio      float64
	preConsumedQuota int
	realtimeChunk    int
	userId           int
	channelId        int
	tokenId          int
	unlimitedQuota   bool
	HandelStatus     bool
	finishOnce       sync.Once

	startTime         time.Time
	firstResponseTime time.Time
	extraBillingData  map[string]ExtraBillingData
}

func NewQuota(c *gin.Context, modelName string, promptTokens int) *Quota {
	isBackupGroup := c.GetBool("is_backupGroup")

	quota := &Quota{
		modelName:      modelName,
		promptTokens:   promptTokens,
		userId:         c.GetInt("id"),
		channelId:      c.GetInt("channel_id"),
		tokenId:        c.GetInt("token_id"),
		unlimitedQuota: c.GetBool("token_unlimited_quota"),
		HandelStatus:   false,
		isBackupGroup:  isBackupGroup, // 记录是否使用备用分组
	}

	quota.price = *model.PricingInstance.GetPrice(quota.modelName)
	quota.groupName = c.GetString("token_group")
	quota.backupGroupName = c.GetString("token_backup_group")
	quota.groupRatio = c.GetFloat64("group_ratio") // 这里的倍率已经在 common.go 中正确设置了
	quota.inputRatio = quota.price.GetInput() * quota.groupRatio
	quota.outputRatio = quota.price.GetOutput() * quota.groupRatio

	return quota

}

func (q *Quota) PreQuotaConsumption() *types.OpenAIErrorWithStatusCode {
	if !q.validPrice() {
		return common.ErrorWrapper(errors.New("invalid quota price"), "invalid_quota_price", http.StatusInternalServerError)
	}
	if q.promptTokens < 0 {
		return common.ErrorWrapper(errors.New("invalid prompt token estimate"), "invalid_quota_estimate", http.StatusBadRequest)
	}
	// Explicitly free models and groups do not require a reservation.
	if q.inputRatio == 0 && q.outputRatio == 0 {
		return nil
	}

	estimate := 1000 * q.inputRatio
	if q.price.Type != model.TimesPriceType {
		if config.PreConsumedQuota < 0 {
			return common.ErrorWrapper(errors.New("invalid pre-consumed quota"), "invalid_quota_estimate", http.StatusInternalServerError)
		}
		estimate = float64(q.promptTokens) * q.inputRatio
	}
	// The exclusive limit remains safe when MaxInt rounds up in float64.
	if math.IsNaN(estimate) || math.IsInf(estimate, 0) || estimate < 0 || estimate >= float64(math.MaxInt)+1 {
		return common.ErrorWrapper(errors.New("quota estimate overflow"), "invalid_quota_estimate", http.StatusBadRequest)
	}
	reservation := int(estimate)
	if q.price.Type != model.TimesPriceType {
		if config.PreConsumedQuota > math.MaxInt-reservation {
			return common.ErrorWrapper(errors.New("quota estimate overflow"), "invalid_quota_estimate", http.StatusBadRequest)
		}
		reservation += config.PreConsumedQuota
	}
	if reservation == 0 {
		reservation = 1
	}
	q.preConsumedQuota = reservation

	userQuota, err := model.CacheGetUserQuota(q.userId)
	if err != nil {
		return common.ErrorWrapper(err, "get_user_quota_failed", http.StatusInternalServerError)
	}

	if userQuota < q.preConsumedQuota {
		return common.ErrorWrapper(errors.New("user quota is not enough"), "insufficient_user_quota", http.StatusPaymentRequired)
	}

	// A high account balance does not imply sufficient finite-token quota.
	// Every positive reservation must pass the model's atomic balance checks.
	if q.preConsumedQuota > 0 {
		err := model.PreConsumeTokenQuotaWithInfo(q.tokenId, q.userId, q.unlimitedQuota, q.preConsumedQuota)
		if err != nil {
			return common.ErrorWrapper(err, "pre_consume_token_quota_failed", http.StatusForbidden)
		}
		q.HandelStatus = true
	}

	return nil
}

// PreRealtimeQuotaConsumption uses the shared paid-request admission boundary.
func (q *Quota) PreRealtimeQuotaConsumption() *types.OpenAIErrorWithStatusCode {
	if err := q.PreQuotaConsumption(); err != nil {
		return err
	}
	q.realtimeChunk = q.preConsumedQuota
	return nil
}

// UpdateUserRealtimeQuota accounts for reported usage before deciding whether
// another reservation window can be funded. A rejected top-up must not erase
// usage that the provider has already incurred.
func (q *Quota) UpdateUserRealtimeQuota(usage *types.UsageEvent, nowUsage *types.UsageEvent) error {
	if nowUsage == nil {
		return nil
	}
	if err := usage.Merge(nowUsage); err != nil {
		return err
	}
	quota := q.GetTotalQuotaByUsage(usage.ToChatUsage())
	if quota < 0 {
		return errors.New("invalid realtime quota")
	}
	if q.inputRatio == 0 && q.outputRatio == 0 {
		return nil
	}
	if !q.HandelStatus {
		return errors.New("realtime reservation is missing")
	}
	// Fixed-price sessions do not incur a new charge for each response.
	if q.price.Type == model.TimesPriceType || quota < q.preConsumedQuota {
		return nil
	}
	if q.realtimeChunk == 0 {
		q.realtimeChunk = q.preConsumedQuota
	}
	if q.realtimeChunk > math.MaxInt-quota {
		return errors.New("realtime quota overflow")
	}
	target := quota + q.realtimeChunk
	if err := model.PreConsumeTokenQuotaWithInfo(q.tokenId, q.userId, q.unlimitedQuota, target-q.preConsumedQuota); err != nil {
		return err
	}
	q.preConsumedQuota = target
	return nil
}

func (q *Quota) completedQuotaConsumption(usage *types.Usage, tokenName string, isStream bool, sourceIp string, ctx context.Context) error {
	quota := q.GetTotalQuotaByUsage(usage)
	if quota < 0 {
		return errors.New("invalid negative settlement quota")
	}

	reserved := 0
	if q.HandelStatus {
		reserved = q.preConsumedQuota
	}
	if quota > 0 || reserved > 0 {
		quotaDelta := quota - reserved
		err := model.PostConsumeTokenQuotaWithInfo(q.tokenId, q.userId, q.unlimitedQuota, quotaDelta)
		if err != nil {
			return errors.New("error consuming token remain quota: " + err.Error())
		}
		if quota > 0 {
			model.UpdateChannelUsedQuota(q.channelId, quota)
		}
	}

	model.RecordConsumeLog(
		ctx,
		q.userId,
		q.channelId,
		usage.PromptTokens,
		usage.CompletionTokens,
		q.modelName,
		tokenName,
		quota,
		"",
		q.getRequestTime(),
		isStream,
		q.GetLogMeta(usage),
		sourceIp,
	)
	model.UpdateUserUsedQuotaAndRequestCount(q.userId, quota)

	return nil
}

func (q *Quota) Undo(c *gin.Context) {
	q.finishOnce.Do(func() {
		if q.HandelStatus {
			// return pre-consumed quota
			err := model.PostConsumeTokenQuotaWithInfo(q.tokenId, q.userId, q.unlimitedQuota, -q.preConsumedQuota)
			if err != nil {
				logger.LogError(c.Request.Context(), "error return pre-consumed quota: "+err.Error())
			}
		}
	})
}

func (q *Quota) Consume(c *gin.Context, usage *types.Usage, isStream bool) {
	// A reservation has one terminal operation. Keep even failed writes terminal:
	// settlement may have committed before a later side effect failed.
	q.finishOnce.Do(func() {
		tokenName := c.GetString("token_name")
		q.startTime = c.GetTime("requestStartTime")
		err := q.completedQuotaConsumption(usage, tokenName, isStream, c.ClientIP(), c.Request.Context())
		if err != nil {
			logger.LogError(c.Request.Context(), err.Error())
		}
	})
}

func (q *Quota) GetInputRatio() float64 {
	return q.inputRatio
}

func (q *Quota) GetLogMeta(usage *types.Usage) map[string]any {
	meta := map[string]any{
		"group_name":        q.groupName,
		"backup_group_name": q.backupGroupName,
		"is_backup_group":   q.isBackupGroup, // 添加是否使用备用分组的标识
		"price_type":        q.price.Type,
		"group_ratio":       q.groupRatio,
		"input_ratio":       q.price.GetInput(),
		"output_ratio":      q.price.GetOutput(),
	}

	firstResponseTime := q.GetFirstResponseTime()
	if firstResponseTime > 0 {
		meta["first_response"] = firstResponseTime
	}

	if usage != nil {
		extraTokens := usage.GetExtraTokens()

		for key, value := range extraTokens {
			meta[key] = value
			extraRatio := q.price.GetExtraRatio(key)
			meta[key+"_ratio"] = extraRatio
		}
	}

	if q.extraBillingData != nil {
		meta["extra_billing"] = q.extraBillingData
	}

	return meta
}

func (q *Quota) getRequestTime() int {
	return int(time.Since(q.startTime).Milliseconds())
}

func (q *Quota) GetFirstResponseTime() int64 {
	// 先判断 firstResponseTime 是否为0
	if q.firstResponseTime.IsZero() {
		return 0
	}

	return q.firstResponseTime.Sub(q.startTime).Milliseconds()
}

func (q *Quota) SetFirstResponseTime(firstResponseTime time.Time) {
	q.firstResponseTime = firstResponseTime
}

type ExtraBillingData struct {
	Type      string  `json:"type"`
	CallCount int     `json:"call_count"`
	Price     float64 `json:"price"`
}

func (q *Quota) GetExtraBillingData(extraBilling map[string]types.ExtraBilling) {
	q.extraBillingData = nil
	if extraBilling == nil {
		return
	}

	extraBillingData := make(map[string]ExtraBillingData)
	for key, value := range extraBilling {
		serviceType := value.ServiceType
		if serviceType == "" {
			serviceType = key
		}
		extraBillingData[key] = ExtraBillingData{
			Type:      value.Type,
			CallCount: value.CallCount,
			Price:     getDefaultExtraServicePrice(serviceType, q.modelName, value.Type),
		}

	}

	if len(extraBillingData) == 0 {
		return
	}

	q.extraBillingData = extraBillingData
}
