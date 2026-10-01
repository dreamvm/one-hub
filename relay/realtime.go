package relay

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"one-api/common"
	"one-api/common/config"
	"one-api/common/logger"
	"one-api/common/requester"
	"one-api/common/utils"
	"one-api/metrics"
	providersBase "one-api/providers/base"
	"one-api/relay/relay_util"
	"one-api/types"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type RelayModeChatRealtime struct {
	relayBase
	userConn       *websocket.Conn
	messageHandler requester.MessageHandler
	providerConn   *websocket.Conn
	quota          *relay_util.Quota
	usage          *types.UsageEvent
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	Subprotocols: []string{"realtime"},
}

func ChatRealtime(c *gin.Context) {
	modelName := c.Query("model")
	if modelName == "" {
		common.AbortWithMessage(c, http.StatusBadRequest, "model_name_required")
		return
	}

	userConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.LogError(c.Request.Context(), "realtime websocket upgrade failed")
		common.AbortWithMessage(c, http.StatusInternalServerError, "upgrade_failed")
		return
	}

	relay := &RelayModeChatRealtime{
		relayBase: relayBase{
			c: c,
		},
		userConn: userConn,
	}
	relay.setOriginalModel(modelName)

	if !relay.getProvider() {
		return
	}

	wsProxy := requester.NewWSProxy(relay.userConn, relay.providerConn, time.Minute*2, relay.messageHandler, relay.usageHandler)
	wsProxy.Start()
	wsProxy.Wait()
	// Both transfer workers have stopped. No usage callback can race final
	// accounting or outlive the Gin request context.
	relay.quota.ReconcileUnfinishedRealtime(relay.usage)
	relay.quota.Consume(relay.c, relay.usage.ToChatUsage(), false)

}

func (r *RelayModeChatRealtime) abortWithMessage(message string) {
	eventErr := types.NewErrorEvent("", "system_error", "system_error", message)

	r.userConn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	r.userConn.WriteMessage(websocket.TextMessage, []byte(eventErr.Error()))
	r.userConn.Close()
}

func (r *RelayModeChatRealtime) getProvider() bool {
	retryTimes := config.RetryTimes
	if retryTimes == 0 {
		retryTimes = 1
	}

	for i := retryTimes; i > 0; i-- {
		// 找不到直接返回
		if err := r.setProvider(r.getOriginalModel()); err != nil {
			r.abortWithMessage(err.Error())
			return false
		}

		realtimeProvider, ok := r.provider.(providersBase.RealtimeInterface)
		if !ok {
			r.abortWithMessage("channel not implemented")
			return false
		}
		channel := r.provider.GetChannel()
		r.quota = relay_util.NewQuota(r.c, r.getModelName(), 0)
		r.usage = &types.UsageEvent{}
		if err := r.quota.PreRealtimeQuotaConsumption(); err != nil {
			r.abortWithMessage(err.Error())
			return false
		}

		providerConn, messageHandler, apiErr := realtimeProvider.CreateChatRealtime(r.modelName)
		if apiErr != nil {
			r.quota.Undo(r.c)
			r.skipChannelIds(channel.Id)
			logger.LogError(r.c.Request.Context(), fmt.Sprintf("using channel #%d(%s) Error: %s to retry (remain times %d)", channel.Id, channel.Name, apiErr.Error(), i))
			metrics.RecordProvider(r.c, apiErr.StatusCode)

			continue
		}

		r.messageHandler = messageHandler
		r.providerConn = providerConn

		if err := r.getRealtimeFirstMessage(); err == nil {
			metrics.RecordProvider(r.c, 200)
			return true
		} else {
			r.providerConn.Close()
			r.quota.ReconcileUnfinishedRealtime(r.usage)
			usage := r.usage.ToChatUsage()
			if usage.HasTokenUsage() || r.quota.NeedsRealtimeReconciliation() {
				r.quota.Consume(r.c, usage, false)
				r.abortWithMessage(err.Error())
				return false
			}
			r.quota.Undo(r.c)
		}
		r.skipChannelIds(channel.Id)
	}

	r.abortWithMessage("get provider failed")
	return false
}

func (r *RelayModeChatRealtime) skipChannelIds(channelId int) {
	skipChannelIds, ok := utils.GetGinValue[[]int](r.c, "skip_channel_ids")
	if !ok {
		skipChannelIds = make([]int, 0)
	}

	skipChannelIds = append(skipChannelIds, channelId)

	r.c.Set("skip_channel_ids", skipChannelIds)
}

func (r *RelayModeChatRealtime) getRealtimeFirstMessage() error {
	timeout := time.Duration(utils.GetOrDefault("connect_timeout", 5)) * time.Second
	r.providerConn.SetReadDeadline(time.Now().Add(timeout))
	messageType, firstMessage, err := r.providerConn.ReadMessage()
	if err != nil {
		return err
	}
	if messageType != websocket.TextMessage {
		return errors.New("unexpected realtime initial message")
	}
	shouldContinue, usage, newMessage, err := r.messageHandler(requester.SupplierMessage, messageType, firstMessage)
	// Capture unknown accounting even when the completion itself is rejected.
	if usage != nil && (usage.MissingUsage || (usage.ResponseStarted && err != nil)) {
		usageErr := r.usageHandler(usage)
		usage = nil
		if err == nil {
			err = usageErr
		}
	}
	if err != nil {
		return err
	}
	if !shouldContinue {
		return errors.New("realtime initial message rejected")
	}
	if usage != nil {
		if err := r.usageHandler(usage); err != nil {
			return err
		}
	}
	if newMessage != nil {
		firstMessage = newMessage
	}
	r.userConn.SetWriteDeadline(time.Now().Add(timeout))
	return r.userConn.WriteMessage(messageType, firstMessage)
}

func (r *RelayModeChatRealtime) usageHandler(usage *types.UsageEvent) error {
	err := r.quota.UpdateUserRealtimeQuota(r.usage, usage)
	if err != nil {
		return types.NewErrorEvent("", "system_error", "system_error", err.Error())
	}

	return nil
}
