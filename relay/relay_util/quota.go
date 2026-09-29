package relay_util

import (
	"errors"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"

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
	requestID        string
	reservationID    string
	reservationReady bool
	terminalMu       sync.Mutex
	terminal         *model.QuotaTerminal
	terminalErr      error

	startTime         time.Time
	firstResponseTime time.Time
	extraBillingData  map[string]ExtraBillingData
}

func NewQuota(c *gin.Context, modelName string, promptTokens int) *Quota {
	isBackupGroup := c.GetBool("is_backupGroup")

	quota := &Quota{
		reservationID:  uuid.NewString(),
		requestID:      c.GetString(logger.RequestIdKey),
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
	// Free requests keep a zero-value receipt without reserving funds.
	if q.inputRatio == 0 && q.outputRatio == 0 {
		if err := model.ReserveQuota(q.reservationID, q.reservationIdentity(), 0); err != nil {
			return common.ErrorWrapper(err, "pre_consume_token_quota_failed", http.StatusForbidden)
		}
		q.reservationReady = true
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
		err := model.ReserveQuota(q.reservationID, q.reservationIdentity(), q.preConsumedQuota)
		if err != nil {
			return common.ErrorWrapper(err, "pre_consume_token_quota_failed", http.StatusForbidden)
		}
		q.HandelStatus = true
		q.reservationReady = true
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
	if err := model.ReserveQuota(q.reservationID, q.reservationIdentity(), target); err != nil {
		return err
	}
	q.preConsumedQuota = target
	return nil
}

func (q *Quota) reservationIdentity() model.QuotaReservationIdentity {
	return model.QuotaReservationIdentity{UserID: q.userId, TokenID: q.tokenId, UnlimitedQuota: q.unlimitedQuota, ChannelID: q.channelId, ModelName: q.modelName, RequestID: q.requestID}
}

func (q *Quota) consumeIntent(c *gin.Context, usage *types.Usage, isStream bool) (*model.QuotaTerminal, error) {
	quota := q.GetTotalQuotaByUsage(usage)
	if quota < 0 {
		return nil, errors.New("invalid settlement quota; reservation requires reconciliation")
	}
	q.startTime = c.GetTime("requestStartTime")
	log := &model.Log{
		CreatedAt: time.Now().Unix(), PromptTokens: usage.PromptTokens, CompletionTokens: usage.CompletionTokens,
		TokenName: c.GetString("token_name"), RequestTime: q.getRequestTime(), IsStream: isStream,
		SourceIp: c.ClientIP(), Metadata: datatypes.NewJSONType(q.GetLogMeta(usage)),
	}
	return &model.QuotaTerminal{Outcome: model.QuotaOutcomeConsume, Quota: quota, Log: log, RecordLog: config.LogConsumeEnabled}, nil
}

func (q *Quota) finish(c *gin.Context, build func() (*model.QuotaTerminal, error)) {
	q.terminalMu.Lock()
	defer q.terminalMu.Unlock()
	if q.terminal == nil && q.terminalErr == nil {
		q.terminal, q.terminalErr = build()
	}
	if q.terminalErr != nil {
		logger.LogError(c.Request.Context(), q.terminalErr.Error())
		return
	}
	// The first terminal choice wins in this object; persisted intent is the
	// cross-process authority. Repeated calls retry that same choice safely.
	if !q.reservationReady {
		if err := model.ReserveQuota(q.reservationID, q.reservationIdentity(), 0); err != nil {
			logger.LogError(c.Request.Context(), "failed to persist quota reservation")
			return
		}
		q.reservationReady = true
	}
	if err := model.SubmitQuotaTerminal(q.reservationID, *q.terminal); err != nil {
		logger.LogError(c.Request.Context(), "failed to persist quota terminal intent")
		return
	}
	if err := model.FinalizeQuotaReservation(q.reservationID); err != nil {
		logger.LogError(c.Request.Context(), "quota terminal intent remains pending recovery")
	}
}

func (q *Quota) Undo(c *gin.Context) {
	q.finish(c, func() (*model.QuotaTerminal, error) {
		return &model.QuotaTerminal{Outcome: model.QuotaOutcomeRefund}, nil
	})
}

func (q *Quota) Consume(c *gin.Context, usage *types.Usage, isStream bool) {
	q.finish(c, func() (*model.QuotaTerminal, error) { return q.consumeIntent(c, usage, isStream) })
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

// ReservationID binds an asynchronous task to this request's immutable charge.
func (q *Quota) ReservationID() string { return q.reservationID }
