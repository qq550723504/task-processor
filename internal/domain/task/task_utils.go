// Package task 提供任务处理工具功能
package task

import (
	"fmt"
	"time"

	"task-processor/internal/common/management/api"
	"task-processor/internal/domain/model"

	"github.com/sirupsen/logrus"
)

// fetchTasksFromAPI 从API获取任务
func (f *TaskFetcher) fetchTasksFromAPI(maxTasks int) ([]api.ProductImportTaskRespDTO, error) {
	// 使用适配器包装管理客户端
	managementClientProvider := f.managementClient
	importTaskClient := managementClientProvider.GetImportTaskClient()

	return importTaskClient.GetPendingAndRetryTasks(
		maxTasks,
		f.config.Management.UserID,
		f.config.Management.StoreIDs,
	)
}

// extractAPITask 从切片中提取API任务（现在直接返回任务，无需类型断言）
func (f *TaskFetcher) extractAPITask(apiTask api.ProductImportTaskRespDTO) *api.ProductImportTaskRespDTO {
	return &apiTask
}

// getStoreInfo 获取店铺信息
func (f *TaskFetcher) getStoreInfo(storeID int64, storeClient interface{}) (*api.StoreRespDTO, error) {
	logrus.Infof("📞 正在获取店铺信息: StoreID=%d", storeID)

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

		logrus.Infof("✅ 成功获取店铺信息: StoreID=%d, Platform=%s, Name=%s",
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
		logrus.Debugf("⏭️ 跳过重复任务: TaskID=%s (已在队列中，提交时间: %v)", taskID, submitTime)
		return true
	}
	return false
}

// markTaskAsProcessing 标记任务为处理中（提交成功后调用）
func (f *TaskFetcher) markTaskAsProcessing(taskID string, apiTaskID int64) {
	// 提交成功后，立即更新任务状态为"处理中"
	f.tasksMutex.Lock()
	f.processingTasks[taskID] = time.Now()
	f.tasksMutex.Unlock()

	// 立即更新API端任务状态，防止重复获取
	f.updateTaskStatusToProcessing(apiTaskID)
}

// markTaskAsProcessingImmediately 立即标记任务为处理中（获取到任务后立即调用）
func (f *TaskFetcher) markTaskAsProcessingImmediately(taskID string, apiTaskID int64) {
	// 立即标记为处理中，防止重复获取
	f.tasksMutex.Lock()
	f.processingTasks[taskID] = time.Now()
	f.tasksMutex.Unlock()

	logrus.Debugf("🔒 任务已立即标记为处理中: TaskID=%s", taskID)

	// 立即更新API端任务状态为处理中
	f.updateTaskStatusToProcessing(apiTaskID)
}

// rollbackProcessingStatus 回滚处理中状态（当任务提交失败时调用）
func (f *TaskFetcher) rollbackProcessingStatus(taskID string) {
	f.tasksMutex.Lock()
	delete(f.processingTasks, taskID)
	f.tasksMutex.Unlock()

	logrus.Debugf("🔄 任务处理中状态已回滚: TaskID=%s", taskID)
}

// updateTaskStatusToProcessing 更新任务状态为"处理中"
func (f *TaskFetcher) updateTaskStatusToProcessing(taskID int64) {
	// 异步更新，避免阻塞任务分发 - 添加panic recovery
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("异步更新任务状态goroutine panic (TaskID=%d): %v", taskID, r)
			}
		}()

		// 使用适配器包装管理客户端
		managementClientProvider := f.managementClient
		importTaskClient := managementClientProvider.GetImportTaskClient()
		if importTaskClient == nil {
			logrus.Error("导入任务客户端未初始化，无法更新任务状态")
			return
		}

		// 更新状态为"处理中"（状态码1）
		updateReq := &api.ProductImportTaskUpdateReqDTO{
			ID:           taskID,
			Status:       model.TaskStatusProcessing.Int16(),
			ErrorMessage: "",
		}

		if err := importTaskClient.UpdateTaskStatus(updateReq); err != nil {
			logrus.Warnf("更新任务状态为处理中失败: TaskID=%d, Error=%v", taskID, err)
		} else {
			logrus.Debugf("✅ 任务状态已更新为处理中: TaskID=%d", taskID)
		}
	}()
}

// getSubmitterKeys 获取所有提交器的键（用于日志输出）
func (f *TaskFetcher) getSubmitterKeys() []string {
	keys := make([]string, 0, len(f.submitters))
	for k := range f.submitters {
		keys = append(keys, k)
	}
	return keys
}
