// Package task 提供任务处理工具功能
package task

import (
	"fmt"
	"task-processor/internal/app/taskstatus"
	"task-processor/internal/core/logger"

	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
)

// fetchTasksFromAPI 从API获取任务
func (f *TaskFetcher) fetchTasksFromAPI(maxTasks int) ([]api.ProductImportTaskRespDTO, error) {
	return NewTaskSource(f).FetchPendingTasks(maxTasks)
}

// extractAPITask 从切片中提取API任务（现在直接返回任务，无需类型断言）
func (f *TaskFetcher) extractAPITask(apiTask api.ProductImportTaskRespDTO) *api.ProductImportTaskRespDTO {
	return &apiTask
}

// getStoreInfo 获取店铺信息
func (f *TaskFetcher) getStoreInfo(storeID int64, storeClient any) (*api.StoreRespDTO, error) {
	logger.GetGlobalLogger("app/task").Infof("📞 正在获取店铺信息: StoreID=%d", storeID)

	// 类型断言为StoreClient接口
	if client, ok := storeClient.(StoreClient); ok {
		storeDTO, err := client.GetStore(storeID)
		if err != nil {
			return nil, fmt.Errorf("获取店铺信息失败: %w", err)
		}

		// 转换StoreDTO为StoreRespDTO
		storeInfo := &api.StoreRespDTO{
			ID:       storeDTO.ID,
			Platform: storeDTO.Platform,
			Name:     storeDTO.Name,
		}

		logger.GetGlobalLogger("app/task").Infof("✅ 成功获取店铺信息: StoreID=%d, Platform=%s, Name=%s",
			storeID, storeInfo.Platform, storeInfo.Name)
		return storeInfo, nil
	}

	return nil, fmt.Errorf("storeClient类型断言失败: %T", storeClient)
}

// isTaskProcessing 检查任务是否正在处理中
func (f *TaskFetcher) isTaskProcessing(taskID string) bool {
	f.tasksMutex.RLock()
	submitTime, isProcessing := f.processingTasks[taskID]
	f.tasksMutex.RUnlock()

	if isProcessing {
		logger.GetGlobalLogger("app/task").Debugf("⏭️ 跳过重复任务: TaskID=%s (已在队列中，提交时间: %v)", taskID, submitTime)
		return true
	}
	return false
}

// rollbackProcessingStatus 回滚处理中状态（当任务提交失败时调用）
func (f *TaskFetcher) rollbackProcessingStatus(taskID string) {
	f.tasksMutex.Lock()
	delete(f.processingTasks, taskID)
	f.tasksMutex.Unlock()

	logger.GetGlobalLogger("app/task").Debugf("🔄 任务处理中状态已回滚: TaskID=%s", taskID)
}

// updateTaskStatusToProcessing 更新任务状态为"处理中"
func (f *TaskFetcher) updateTaskStatusToProcessing(taskID int64) {
	statusService := taskstatus.NewService("app/task_fetcher", func() taskstatus.ImportTaskStatusClient {
		if f.managementClient == nil {
			return nil
		}
		return f.managementClient.GetImportTaskClient()
	})
	statusService.UpdateAsync(taskID, model.TaskStatusProcessing, "")
}

// getSubmitterKeys 获取所有提交器的键（用于日志输出）
func (f *TaskFetcher) getSubmitterKeys() []string {
	keys := make([]string, 0, len(f.submitters))
	for k := range f.submitters {
		keys = append(keys, k)
	}
	return keys
}
