package task

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"one-api/common/config"
	"one-api/common/logger"
	"one-api/metrics"
	"one-api/model"
	"one-api/relay/relay_util"
	"one-api/relay/task/base"
	"one-api/types"
)

func RelayTaskSubmit(c *gin.Context) {
	var taskErr *base.TaskError
	taskAdaptor, err := GetTaskAdaptor(GetRelayMode(c), c)
	if err != nil {
		taskErr = base.StringTaskError(http.StatusBadRequest, "adaptor_not_found", "adaptor not found", true)
		c.JSON(http.StatusBadRequest, taskErr)
		return
	}

	taskErr = taskAdaptor.Init()
	if taskErr != nil {
		taskAdaptor.HandleError(taskErr)
		return
	}

	taskErr = taskAdaptor.SetProvider()
	if taskErr != nil {
		taskAdaptor.HandleError(taskErr)
		return
	}

	quotaInstance := relay_util.NewQuota(c, taskAdaptor.GetModelName(), 1000)
	if errWithOA := quotaInstance.PreQuotaConsumption(); errWithOA != nil {
		taskAdaptor.HandleError(base.OpenAIErrToTaskErr(errWithOA))
		return
	}
	// Each provider attempt owns its receipt, including captured channel/pricing.
	// The current attempt is released if no successful submission consumes it.
	defer func() { quotaInstance.Undo(c) }()

	taskErr = taskAdaptor.Relay()
	if taskErr == nil {
		CompletedTask(quotaInstance, taskAdaptor, c)
		// 返回结果
		taskAdaptor.GinResponse()
		metrics.RecordProvider(c, 200)
		return
	}

	retryTimes := config.RetryTimes

	if !taskAdaptor.ShouldRetry(c, taskErr) {
		logger.LogError(c.Request.Context(), fmt.Sprintf("relay error happen, status code is %d, won't retry in this case", taskErr.StatusCode))
		retryTimes = 0
	}

	channel := taskAdaptor.GetProvider().GetChannel()
	for i := retryTimes; i > 0; i-- {
		quotaInstance.Undo(c)
		model.ChannelGroup.SetCooldowns(channel.Id, taskAdaptor.GetModelName())
		taskErr = taskAdaptor.SetProvider()
		if taskErr != nil {
			continue
		}

		channel = taskAdaptor.GetProvider().GetChannel()
		logger.LogError(c.Request.Context(), fmt.Sprintf("using channel #%d(%s) to retry (remain times %d)", channel.Id, channel.Name, i))

		quotaInstance = relay_util.NewQuota(c, taskAdaptor.GetModelName(), 1000)
		if errWithOA := quotaInstance.PreQuotaConsumption(); errWithOA != nil {
			taskErr = base.OpenAIErrToTaskErr(errWithOA)
			break
		}

		taskErr = taskAdaptor.Relay()
		if taskErr == nil {
			CompletedTask(quotaInstance, taskAdaptor, c)
			taskAdaptor.GinResponse()
			metrics.RecordProvider(c, 200)
			return
		}

		if !taskAdaptor.ShouldRetry(c, taskErr) {
			break
		}

	}

	if taskErr != nil {
		taskAdaptor.HandleError(taskErr)
	}

}

func CompletedTask(quotaInstance *relay_util.Quota, taskAdaptor base.TaskInterface, c *gin.Context) {
	quotaInstance.Consume(c, &types.Usage{CompletionTokens: 0, PromptTokens: 1, TotalTokens: 1}, false)

	task := taskAdaptor.GetTask()
	task.ReservationID = quotaInstance.ReservationID()
	task.Quota = quotaInstance.GetTotalQuotaByUsage(&types.Usage{PromptTokens: 1, TotalTokens: 1})

	err := task.Insert()
	if err != nil {
		logger.SysError(fmt.Sprintf("task error: %s", err.Error()))
	}

	// 激活任务
	ActivateUpdateTaskBulk()
}

func GetRelayMode(c *gin.Context) int {
	relayMode := config.RelayModeUnknown
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/suno") {
		relayMode = config.RelayModeSuno
	} else if strings.HasPrefix(path, "/kling") {
		relayMode = config.RelayModeKling
	}

	return relayMode
}
