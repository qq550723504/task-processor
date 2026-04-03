// Package task provides helper methods for task fetching and claim lifecycle.
package task

import (
	"fmt"

	"task-processor/internal/app/taskstatus"
	"task-processor/internal/core/logger"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
)

// fetchTasksFromAPI fetches candidate tasks from management.
func (f *TaskFetcher) fetchTasksFromAPI(maxTasks int) ([]api.ProductImportTaskRespDTO, error) {
	return NewTaskSource(f).FetchPendingTasks(maxTasks)
}

// extractAPITask returns the API task as a pointer for downstream helpers.
func (f *TaskFetcher) extractAPITask(apiTask api.ProductImportTaskRespDTO) *api.ProductImportTaskRespDTO {
	return &apiTask
}

// getStoreInfo loads store information from the management client.
func (f *TaskFetcher) getStoreInfo(storeID int64, storeClient any) (*api.StoreRespDTO, error) {
	logger.GetGlobalLogger("app/task").Infof("正在获取店铺信息: StoreID=%d", storeID)

	if client, ok := storeClient.(StoreClient); ok {
		storeDTO, err := client.GetStore(storeID)
		if err != nil {
			return nil, fmt.Errorf("获取店铺信息失败: %w", err)
		}

		storeInfo := &api.StoreRespDTO{
			ID:       storeDTO.ID,
			Platform: storeDTO.Platform,
			Name:     storeDTO.Name,
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
		return f.managementClient.GetImportTaskClient()
	})
}

// updateTaskStatusToProcessing persists a successful claim to management.
func (f *TaskFetcher) updateTaskStatusToProcessing(taskID int64, fromCode int16) error {
	statusService := f.newTaskStatusService("app/task_fetcher")
	return statusService.TransitionFromCodeSync(taskID, fromCode, model.TaskStatusProcessing, "")
}

// rollbackClaimState compensates a claimed task back to its original remote status.
func (f *TaskFetcher) rollbackClaimState(taskID string, apiTask *api.ProductImportTaskRespDTO, reason string) {
	defer f.rollbackProcessingStatus(taskID)

	if apiTask == nil {
		return
	}

	originalStatus, err := model.ParseTaskStatus(apiTask.Status)
	if err != nil {
		logger.GetGlobalLogger("app/task").WithError(err).Warnf(
			"无法回滚任务远端状态: TaskID=%s, Reason=%s, StatusCode=%d, StatusKey=%s, CanonicalStatus=%s",
			taskID,
			reason,
			apiTask.Status,
			apiTask.StatusKey,
			apiTask.CanonicalStatus,
		)
		return
	}

	statusService := f.newTaskStatusService("app/task_fetcher")
	if err := statusService.UpdateSync(apiTask.ID, originalStatus, apiTask.ErrorMessage); err != nil {
		logger.GetGlobalLogger("app/task").WithError(err).Warnf(
			"回滚任务远端状态失败: TaskID=%s, TargetStatus=%s, Reason=%s, OriginalStatusKey=%s, OriginalCanonicalStatus=%s",
			taskID,
			originalStatus.String(),
			reason,
			apiTask.StatusKey,
			apiTask.CanonicalStatus,
		)
		return
	}

	logger.GetGlobalLogger("app/task").Infof(
		"任务 claim 后补偿回滚成功: TaskID=%s, TargetStatus=%s, Reason=%s, OriginalStatusKey=%s, OriginalCanonicalStatus=%s",
		taskID,
		originalStatus.String(),
		reason,
		apiTask.StatusKey,
		apiTask.CanonicalStatus,
	)
	f.removeClaimJournalEntry(apiTask.ID)
}

// getSubmitterKeys returns the registered submitter keys for logging.
func (f *TaskFetcher) getSubmitterKeys() []string {
	keys := make([]string, 0, len(f.submitters))
	for k := range f.submitters {
		keys = append(keys, k)
	}
	return keys
}
