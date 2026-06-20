// Package task provides helper methods for task fetching and claim lifecycle.
package task

import (
	"fmt"

	"task-processor/internal/app/taskstatus"
	"task-processor/internal/core/logger"
	"task-processor/internal/listingruntime"
	"task-processor/internal/model"
)

type runtimeImportTaskStatusClient struct {
	updateFn func(req *listingruntime.TaskStatusUpdate) error
}

func (c runtimeImportTaskStatusClient) UpdateTaskStatus(req *listingruntime.TaskStatusUpdate) error {
	if c.updateFn == nil {
		return fmt.Errorf("runtime import task status updater is not configured")
	}
	return c.updateFn(req)
}

// fetchTasksFromAPI fetches candidate tasks from management.
func (f *TaskFetcher) fetchTasksFromAPI(maxTasks int) ([]ImportTaskRecord, error) {
	return NewTaskSource(f).FetchPendingTasks(maxTasks)
}

func toImportTaskRecord(apiTask listingruntime.ImportTask) ImportTaskRecord {
	return ImportTaskRecord{
		ID:               apiTask.ID,
		TenantID:         apiTask.TenantID,
		StoreID:          apiTask.StoreID,
		Platform:         apiTask.Platform,
		Region:           apiTask.Region,
		CategoryID:       apiTask.CategoryID,
		ProductID:        apiTask.ProductID,
		Status:           apiTask.Status,
		ErrorMessage:     apiTask.ErrorMessage,
		RetryCount:       apiTask.RetryCount,
		Priority:         apiTask.Priority,
		CreateTimeMillis: apiTask.CreateTime,
		Creator:          apiTask.Creator,
		StatusKey:        apiTask.StatusKey,
		CanonicalStatus:  apiTask.CanonicalStatus,
	}
}

// getStoreInfo loads store information from the management client.
func toStoreInfo(storeDTO *listingruntime.StoreInfo) *StoreInfo {
	if storeDTO == nil {
		return nil
	}
	return &StoreInfo{
		ID:             storeDTO.ID,
		TenantID:       storeDTO.TenantID,
		StoreID:        storeDTO.StoreID,
		Platform:       storeDTO.Platform,
		Name:           storeDTO.Name,
		Region:         storeDTO.Region,
		ShopType:       storeDTO.ShopType,
		PriceType:      storeDTO.PriceType,
		DailyLimit:     storeDTO.DailyLimit,
		DailyLimitType: storeDTO.DailyLimitType,
		EnableDraft:    storeDTO.EnableDraft,
	}
}

func (f *TaskFetcher) getStoreInfo(storeID int64, storeClient any) (*StoreInfo, error) {
	logger.GetGlobalLogger("app/task").Infof("正在获取店铺信息: StoreID=%d", storeID)

	if client, ok := storeClient.(StoreClient); ok {
		storeDTO, err := client.GetStore(storeID)
		if err != nil {
			return nil, fmt.Errorf("获取店铺信息失败: %w", err)
		}
		storeInfo := toStoreInfo(storeDTO)
		if storeInfo == nil {
			return nil, fmt.Errorf("店铺信息为空")
		}

		logger.GetGlobalLogger("app/task").Infof("成功获取店铺信息: StoreID=%d, Platform=%s, Name=%s",
			storeID, storeInfo.Platform, storeInfo.Name)
		return storeInfo, nil
	}

	return nil, fmt.Errorf("storeClient类型断言失败: %T", storeClient)
}

// isTaskProcessing checks whether a task is already being processed locally.
func (f *TaskFetcher) isTaskProcessing(taskID string) bool {
	f.tasksMutex.RLock()
	submitTime, isProcessing := f.processingTasks[taskID]
	f.tasksMutex.RUnlock()

	if isProcessing {
		logger.GetGlobalLogger("app/task").Debugf("跳过重复任务: TaskID=%s (已在队列中, 提交时间: %v)", taskID, submitTime)
		return true
	}
	return false
}

// rollbackProcessingStatus only reverts the local de-duplication marker.
func (f *TaskFetcher) rollbackProcessingStatus(taskID string) {
	f.tasksMutex.Lock()
	delete(f.processingTasks, taskID)
	f.tasksMutex.Unlock()

	logger.GetGlobalLogger("app/task").Debugf("任务本地 processing 标记已回滚: TaskID=%s", taskID)
}

func (f *TaskFetcher) newTaskStatusService(component string) *taskstatus.Service {
	if f != nil && f.statusServiceFactory != nil {
		return f.statusServiceFactory(component)
	}

	return taskstatus.NewService(component, func() taskstatus.ImportTaskStatusClient {
		if f == nil || f.managementClient == nil {
			return nil
		}
		return runtimeImportTaskStatusClient{
			updateFn: f.managementClient.UpdateRuntimeTaskStatus,
		}
	})
}

// updateTaskStatusToProcessing persists a successful claim to management.
func (f *TaskFetcher) updateTaskStatusToProcessing(taskID int64, fromCode int16) error {
	statusService := f.newTaskStatusService("app/task_fetcher")
	return statusService.TransitionFromCodeSync(taskID, fromCode, model.TaskStatusProcessing, "")
}

// rollbackClaimState compensates a claimed task back to its original remote status.
func (f *TaskFetcher) rollbackClaimState(taskID string, task *ImportTaskRecord, reason string) {
	defer f.rollbackProcessingStatus(taskID)

	if task == nil {
		return
	}

	originalStatus, err := model.ParseTaskStatus(task.Status)
	if err != nil {
		logger.GetGlobalLogger("app/task").WithError(err).Warnf(
			"无法回滚任务远端状态: TaskID=%s, Reason=%s, StatusCode=%d, StatusKey=%s, CanonicalStatus=%s",
			taskID,
			reason,
			task.Status,
			task.StatusKey,
			task.CanonicalStatus,
		)
		return
	}

	statusService := f.newTaskStatusService("app/task_fetcher")
	if err := statusService.UpdateSync(task.ID, originalStatus, task.ErrorMessage); err != nil {
		logger.GetGlobalLogger("app/task").WithError(err).Warnf(
			"回滚任务远端状态失败: TaskID=%s, TargetStatus=%s, Reason=%s, OriginalStatusKey=%s, OriginalCanonicalStatus=%s",
			taskID,
			originalStatus.String(),
			reason,
			task.StatusKey,
			task.CanonicalStatus,
		)
		return
	}

	logger.GetGlobalLogger("app/task").Infof(
		"任务 claim 后补偿回滚成功: TaskID=%s, TargetStatus=%s, Reason=%s, OriginalStatusKey=%s, OriginalCanonicalStatus=%s",
		taskID,
		originalStatus.String(),
		reason,
		task.StatusKey,
		task.CanonicalStatus,
	)
	f.removeClaimJournalEntry(task.ID)
}

// getSubmitterKeys returns the registered submitter keys for logging.
func (f *TaskFetcher) getSubmitterKeys() []string {
	keys := make([]string, 0, len(f.submitters))
	for k := range f.submitters {
		keys = append(keys, k)
	}
	return keys
}
