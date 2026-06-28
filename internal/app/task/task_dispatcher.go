package task

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"task-processor/internal/core/logger"
	"task-processor/internal/infra/worker"
	"task-processor/internal/listingruntime"
	types "task-processor/internal/model"
)

type TaskDispatcher struct {
	fetcher *TaskFetcher
}

func NewTaskDispatcher(fetcher *TaskFetcher) *TaskDispatcher {
	return &TaskDispatcher{fetcher: fetcher}
}

func (d *TaskDispatcher) Dispatch(ctx context.Context, task *ImportTaskRecord, storeInfo *listingruntime.StoreInfo) (success bool, isQueueFull bool) {
	if d == nil || d.fetcher == nil || task == nil || storeInfo == nil {
		return false, false
	}

	internalTask := types.Task{
		ID:         task.ID,
		TenantID:   task.TenantID,
		ProductID:  task.ProductID,
		Platform:   task.Platform,
		Region:     task.Region,
		StoreID:    task.StoreID,
		CategoryID: task.CategoryID,
		CreateTime: task.CreateTimeMillis / 1000,
		RetryCount: task.RetryCount,
		Priority:   task.Priority,
		Creator:    task.Creator,
	}

	taskData, err := json.Marshal(internalTask)
	if err != nil {
		logger.GetGlobalLogger("app/task").Errorf("序列化任务失败: %v", err)
		return false, false
	}

	platform := strings.ToLower(storeInfo.Platform)
	submitter, exists := d.fetcher.submitters[platform]
	if !exists {
		logger.GetGlobalLogger("app/task").Errorf("❌ 未找到平台处理器: TaskID=%d, StoreID=%d, Platform=%s, 可用平台=%v",
			internalTask.ID, task.StoreID, platform, d.fetcher.getSubmitterKeys())
		return false, false
	}

	if err := submitter.SubmitTask(ctx, string(taskData)); err != nil {
		if errors.Is(err, worker.ErrQueueFull) {
			logger.GetGlobalLogger("app/task").Debugf("[%s] 队列已满，任务将重试: TaskID=%d", platform, internalTask.ID)
			return false, true
		}
		logger.GetGlobalLogger("app/task").Errorf("[%s] 提交失败: TaskID=%d, Error=%v", platform, internalTask.ID, err)
		return false, false
	}

	logger.GetGlobalLogger("app/task").Debugf("[%s] 任务已提交: ID=%d, ProductID=%s", platform, internalTask.ID, internalTask.ProductID)
	return true, false
}
