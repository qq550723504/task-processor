package management

import (
	"fmt"
	"net/http"
	"task-processor/internal/core/logger"
	"task-processor/internal/infra/clients/management/api"
)

// ImportTaskAPIClient 导入任务API客户端实现
type ImportTaskAPIClient struct {
	*ManagementAPIClient
	localDataProvider *LocalDataProvider
}

// GetPendingAndRetryTasks 获取待处理及待重试的任务列表
func (m *ImportTaskAPIClient) GetPendingAndRetryTasks(limit int, userId int64, storeIds []int64) ([]api.ProductImportTaskRespDTO, error) {
	logger.GetGlobalLogger("infra/clients").Warn("[GetPendingAndRetryTasks] legacy polling API is deprecated; prefer RabbitMQ-driven task dispatch")
	if m.localDataProvider != nil && m.localDataProvider.HasDB() {
		if tasks, handled, err := m.localDataProvider.GetPendingAndRetryTasks(limit, userId, storeIds); err != nil || handled {
			return tasks, err
		}
	}
	url := fmt.Sprintf("%s/rpc-api/listing/import-task/list-pending-and-retry?limit=%d", m.baseURL, limit)

	if userId > 0 {
		url = fmt.Sprintf("%s&userId=%d", url, userId)
	}

	if len(storeIds) > 0 {
		storeIdsStr := ""
		for i, storeId := range storeIds {
			if i > 0 {
				storeIdsStr += ","
			}
			storeIdsStr += fmt.Sprintf("%d", storeId)
		}
		url = fmt.Sprintf("%s&storeIds=%s", url, storeIdsStr)
	}

	var result APIResponse
	var tasks []api.ProductImportTaskRespDTO
	result.Data = &tasks

	if err := m.apiRequest(http.MethodGet, url, nil, &result); err != nil {
		return nil, fmt.Errorf("请求导入任务失败: %w", err)
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, fmt.Errorf("处理API响应失败: %w", err)
	}

	if tasksPtr, ok := result.Data.(*[]api.ProductImportTaskRespDTO); ok {
		taskCount := len(*tasksPtr)
		if taskCount == 0 {
			logger.GetGlobalLogger("infra/clients").Warnf("[GetPendingAndRetryTasks] 未获取到任何任务！请求参数: limit=%d, userId=%d, storeIds=%v",
				limit, userId, storeIds)
		} else {
			firstTask := (*tasksPtr)[0]
			logger.GetGlobalLogger("infra/clients").Debugf(
				"[GetPendingAndRetryTasks] 获取任务成功: count=%d, firstTaskID=%d, firstStatusCode=%d, firstStatusKey=%s, firstCanonicalStatus=%s",
				taskCount,
				firstTask.ID,
				firstTask.Status,
				firstTask.StatusKey,
				firstTask.CanonicalStatus,
			)
		}
		return *tasksPtr, nil
	}

	return nil, fmt.Errorf("无法解析任务列表")
}

// UpdateTaskStatus 更新任务状态
func (m *ImportTaskAPIClient) UpdateTaskStatus(req *api.ProductImportTaskUpdateReqDTO) error {
	if m.localDataProvider != nil && m.localDataProvider.HasDB() {
		handled, err := m.localDataProvider.UpdateImportTaskStatus(req)
		if err != nil {
			return fmt.Errorf("更新任务状态失败: %w", err)
		}
		if handled {
			return nil
		}
	}
	url := fmt.Sprintf("%s/rpc-api/listing/import-task/update-status", m.baseURL)

	success, err := getTypedResult[bool](m.ManagementAPIClient, http.MethodPost, url, req)
	if err != nil {
		return fmt.Errorf("更新任务状态失败: %w", err)
	}
	if !success {
		return fmt.Errorf("管理端拒绝更新任务状态: taskId=%d, status=%d, expectedCurrentStatus=%v",
			req.ID, req.Status, req.ExpectedCurrentStatus)
	}

	return nil
}
