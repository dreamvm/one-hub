// Author: Calcium-Ion
// GitHub: https://github.com/Calcium-Ion/new-api
// Path: controller/midjourney.go
package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"one-api/common"
	"one-api/common/logger"
	"one-api/common/requester"
	"one-api/common/utils"
	"one-api/model"
	provider "one-api/providers/midjourney"
)

var (
	taskActive int32 = 0
	lock       sync.Mutex
	cond       = sync.NewCond(&lock)
)

func InitMidjourneyTask() {
	common.SafeGoroutine(func() {
		midjourneyTask()
	})

	ActivateUpdateMidjourneyTaskBulk()
}

func midjourneyTask() {
	for {
		lock.Lock()
		for atomic.LoadInt32(&taskActive) == 0 {
			cond.Wait() // 等待激活信号
		}
		lock.Unlock()
		UpdateMidjourneyTaskBulk()
	}
}

func ActivateUpdateMidjourneyTaskBulk() {
	if atomic.LoadInt32(&taskActive) == 1 {
		return
	}

	lock.Lock()
	atomic.StoreInt32(&taskActive, 1)
	cond.Signal() // 通知等待的任务
	lock.Unlock()
}

func DeactivateMidjourneyTaskBulk() {
	if atomic.LoadInt32(&taskActive) == 0 {
		return
	}

	lock.Lock()
	atomic.StoreInt32(&taskActive, 0)
	lock.Unlock()
}

func UpdateMidjourneyTaskBulk() {
	ctx := context.WithValue(context.Background(), logger.RequestIdKey, "MidjourneyTask")
	for {
		logger.LogInfo(ctx, "running")

		tasks := model.GetAllUnFinishTasks()

		// 如果没有未完成的任务，则等待
		if len(tasks) == 0 {
			DeactivateMidjourneyTaskBulk()
			logger.LogInfo(ctx, "no tasks, waiting...")
			return
		}

		logger.LogWarn(ctx, fmt.Sprintf("检测到未完成的任务数有: %v", len(tasks)))
		taskChannelM, nullTaskIds := groupMidjourneyTasks(tasks)
		if len(nullTaskIds) > 0 {
			err := model.MjBulkUpdateByTaskIds(nullTaskIds, map[string]any{
				"status":   "FAILURE",
				"progress": "100%",
			})
			if err != nil {
				logger.LogError(ctx, fmt.Sprintf("Fix null mj_id task error: %v", err))
			} else {
				logger.LogInfo(ctx, fmt.Sprintf("Fix null mj_id task success: %v", nullTaskIds))
			}
		}
		if len(taskChannelM) == 0 {
			continue
		}

		for channelId, taskM := range taskChannelM {
			taskIds := make([]string, 0, len(taskM))
			var rowIDs []int
			for id, rows := range taskM {
				taskIds = append(taskIds, id)
				for _, row := range rows {
					rowIDs = append(rowIDs, row.Id)
				}
			}
			logger.LogWarn(ctx, fmt.Sprintf("渠道 #%d 未完成的任务有: %d", channelId, len(taskIds)))
			if len(taskIds) == 0 {
				continue
			}
			midjourneyChannel := model.ChannelGroup.GetChannel(channelId)
			if midjourneyChannel == nil {
				err := model.MjBulkUpdateByTaskIds(rowIDs, map[string]any{
					"fail_reason": fmt.Sprintf("获取渠道信息失败，请联系管理员，渠道ID：%d", channelId),
					"status":      "FAILURE",
					"progress":    "100%",
				})
				logger.LogError(ctx, fmt.Sprintf("UpdateMidjourneyTask error: %v", err))
				continue
			}

			err := mjTaskHandler(midjourneyChannel, taskIds, taskM)
			if err != nil {
				logger.LogError(ctx, fmt.Sprintf("MjTaskHandler error: %v", err))
			}
		}
		time.Sleep(time.Duration(15) * time.Second)
	}
}

func MjTaskHandler(midjourneyChannel *model.Channel, taskIds []string, taskM map[string]*model.Midjourney) error {
	rows := make(map[string][]*model.Midjourney, len(taskM))
	for id, task := range taskM {
		rows[id] = []*model.Midjourney{task}
	}
	return mjTaskHandler(midjourneyChannel, taskIds, rows)
}

func mjTaskHandler(midjourneyChannel *model.Channel, taskIds []string, taskM map[string][]*model.Midjourney) error {
	if midjourneyChannel == nil || midjourneyChannel.BaseURL == nil || len(taskIds) == 0 {
		return errors.New("invalid midjourney channel or task batch")
	}
	requested := make(map[string]bool, len(taskIds))
	for _, id := range taskIds {
		if id == "" || requested[id] || len(taskM[id]) == 0 {
			return errors.New("invalid midjourney task binding")
		}
		requested[id] = true
		for _, task := range taskM[id] {
			if task == nil || task.Id <= 0 || task.MjId != id || task.ChannelId != midjourneyChannel.Id {
				return errors.New("invalid midjourney task binding")
			}
		}
	}
	requestUrl := fmt.Sprintf("%s/mj/task/list-by-condition", *midjourneyChannel.BaseURL)

	body, _ := json.Marshal(map[string]any{
		"ids": taskIds,
	})
	req, err := http.NewRequest("POST", requestUrl, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("get task error: %v", err)
	}
	// 设置超时时间
	timeout := time.Second * 5
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if midjourneyChannel.Proxy != nil {
		ctx = utils.SetProxy(*midjourneyChannel.Proxy, ctx)
	}
	// 使用带有超时的 context 创建新的请求
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("mj-api-secret", midjourneyChannel.Key)
	resp, err := requester.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("get task do req error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get task status code: %d", resp.StatusCode)
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("get task parse body error: %v", err)
	}
	var responseItems []provider.MidjourneyDto
	err = json.Unmarshal(responseBody, &responseItems)
	if err != nil {
		return fmt.Errorf("get task parse body error2: %v, body: %s", err, string(responseBody))
	}

	// Validate the whole response before writing any task. Provider IDs are only
	// meaningful within the channel and the requested batch, not globally.
	seen := make(map[string]provider.MidjourneyDto, len(responseItems))
	uniqueItems := make([]provider.MidjourneyDto, 0, len(responseItems))
	for _, item := range responseItems {
		if !requested[item.MjId] {
			return errors.New("unbound midjourney response")
		}
		if previous, exists := seen[item.MjId]; exists {
			if !reflect.DeepEqual(previous, item) {
				return errors.New("conflicting midjourney response")
			}
			continue
		}
		seen[item.MjId] = item
		uniqueItems = append(uniqueItems, item)
	}
	for _, item := range uniqueItems {
		for _, task := range taskM[item.MjId] {
			responseItem := item
			useTime := (time.Now().UnixNano() / int64(time.Millisecond)) - task.SubmitTime
			// 如果时间超过一小时，且进度不是100%，则认为任务失败
			if useTime > 3600000 && task.Progress != "100%" {
				responseItem.FailReason = "上游任务超时（超过1小时）"
				responseItem.Status = "FAILURE"
			}

			if !checkMjTaskNeedUpdate(task, responseItem) {
				continue
			}
			task.Code = 1
			task.Progress = responseItem.Progress
			task.PromptEn = responseItem.PromptEn
			task.State = responseItem.State
			task.SubmitTime = responseItem.SubmitTime
			task.StartTime = responseItem.StartTime
			task.FinishTime = responseItem.FinishTime
			task.ImageUrl = responseItem.ImageUrl
			task.Status = responseItem.Status
			task.FailReason = responseItem.FailReason
			if responseItem.Properties != nil {
				propertiesStr, _ := json.Marshal(responseItem.Properties)
				task.Properties = string(propertiesStr)
			}
			if responseItem.Buttons != nil {
				buttonStr, _ := json.Marshal(responseItem.Buttons)
				task.Buttons = string(buttonStr)
			}

			failed := task.Status == "FAILURE" || (task.Progress != "100%" && responseItem.FailReason != "")
			if failed {
				task.Progress = "100%"
			}
			err = task.UpdateFromPoll(failed)
			if err != nil {
				logger.LogError(ctx, "UpdateMidjourneyTask task error: "+err.Error())
			}
		}
	}

	return nil
}

func checkMjTaskNeedUpdate(oldTask *model.Midjourney, newTask provider.MidjourneyDto) bool {
	if oldTask.Code != 1 {
		return true
	}
	if oldTask.Progress != newTask.Progress {
		return true
	}
	if oldTask.PromptEn != newTask.PromptEn {
		return true
	}
	if oldTask.State != newTask.State {
		return true
	}
	if oldTask.SubmitTime != newTask.SubmitTime {
		return true
	}
	if oldTask.StartTime != newTask.StartTime {
		return true
	}
	if oldTask.FinishTime != newTask.FinishTime {
		return true
	}
	if oldTask.ImageUrl != newTask.ImageUrl {
		return true
	}
	if oldTask.Status != newTask.Status {
		return true
	}
	if oldTask.FailReason != newTask.FailReason {
		return true
	}
	if oldTask.FinishTime != newTask.FinishTime {
		return true
	}
	if oldTask.Progress != "100%" && newTask.FailReason != "" {
		return true
	}

	return false
}

func GetAllMidjourney(c *gin.Context) {
	var params model.MJTaskQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	midjourneys, err := model.GetAllMJTasks(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    midjourneys,
	})
}

func GetUserMidjourney(c *gin.Context) {
	userId := c.GetInt("id")
	tokenId := c.GetInt("token_id")
	var params model.MJTaskQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	if tokenId > 0 {
		params.TokenID = tokenId
	}

	midjourneys, err := model.GetAllUserMJTask(userId, &params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    midjourneys,
	})
}
