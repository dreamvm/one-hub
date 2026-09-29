package task

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"one-api/common"
	"one-api/common/logger"
	"one-api/model"
)

var (
	taskActive int32 = 0
	lock       sync.Mutex
	cond       = sync.NewCond(&lock)
)

func InitTask() {
	common.SafeGoroutine(func() {
		Task()
	})

	ActivateUpdateTaskBulk()
}

func Task() {
	for {
		lock.Lock()
		for atomic.LoadInt32(&taskActive) == 0 {
			cond.Wait() // 等待激活信号
		}
		lock.Unlock()
		UpdateTaskBulk()
	}
}

func ActivateUpdateTaskBulk() {
	if atomic.LoadInt32(&taskActive) == 1 {
		return
	}

	lock.Lock()
	atomic.StoreInt32(&taskActive, 1)
	cond.Signal() // 通知等待的任务
	lock.Unlock()
}

func DeactivateTask() {
	if atomic.LoadInt32(&taskActive) == 0 {
		return
	}

	lock.Lock()
	atomic.StoreInt32(&taskActive, 0)
	lock.Unlock()
}

func UpdateTaskBulk() {
	ctx := context.WithValue(context.Background(), logger.RequestIdKey, "Task")
	for {
		logger.LogInfo(ctx, "running")
		allTasks := model.GetAllUnFinishSyncTasks(500)
		platformTask := make(map[string][]*model.Task)

		if len(allTasks) == 0 {
			DeactivateTask()
			logger.LogInfo(ctx, "no tasks, waiting...")
			return
		}

		for _, t := range allTasks {
			platformTask[t.Platform] = append(platformTask[t.Platform], t)
		}
		for platform, tasks := range platformTask {
			if len(tasks) == 0 {
				continue
			}
			batches, nullTaskIds := groupTaskPollBatches(tasks)

			if len(nullTaskIds) > 0 {
				err := model.TaskBulkUpdateByID(nullTaskIds, map[string]any{
					"status":   "FAILURE",
					"progress": 100,
				})
				if err != nil {
					logger.LogError(ctx, fmt.Sprintf("Fix null task_id task error: %v", err))
				} else {
					logger.LogInfo(ctx, fmt.Sprintf("Fix null task_id task success: %v", nullTaskIds))
				}
			}
			for _, batch := range batches {
				UpdateTaskByPlatform(ctx, platform, map[int][]string{batch.channelID: batch.ids}, batch.tasks)
			}
		}
		time.Sleep(time.Duration(15) * time.Second)
	}
}

func UpdateTaskByPlatform(ctx context.Context,
	platform string, taskChannelM map[int][]string, taskM map[string]*model.Task) {
	taskAdaptor, err := GetTaskAdaptorByPlatform(platform)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("GetTaskAdaptorByPlatform error: %v", err))
		return
	}

	taskAdaptor.UpdateTaskStatus(ctx, taskChannelM, taskM)
}

// Duplicate external IDs, even within one channel, must retain every local row.
type taskPollBatch struct {
	channelID int
	ids       []string
	tasks     map[string]*model.Task
}

func groupTaskPollBatches(tasks []*model.Task) ([]taskPollBatch, []int64) {
	var batches []taskPollBatch
	var missing []int64
	for _, task := range tasks {
		if task.TaskID == "" {
			missing = append(missing, task.ID)
			continue
		}
		index := -1
		for i := range batches {
			if batches[i].channelID == task.ChannelId && batches[i].tasks[task.TaskID] == nil {
				index = i
				break
			}
		}
		if index < 0 {
			batches = append(batches, taskPollBatch{channelID: task.ChannelId, tasks: make(map[string]*model.Task)})
			index = len(batches) - 1
		}
		batches[index].ids = append(batches[index].ids, task.TaskID)
		batches[index].tasks[task.TaskID] = task
	}
	return batches, missing
}
