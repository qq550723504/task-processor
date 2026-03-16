package management

import (
	"fmt"
	"net/http"
	"task-processor/internal/infra/clients/management/api"

	"github.com/sirupsen/logrus"
)

// ImportTaskAPIClient 导入任务API客户端实现
type ImportTaskAPIClient struct {
	*ManagementAPIClient
}

// GetPendingAndRetryTasks 获取待处理及待重试的任务列表
func (m *ImportTaskAPIClient) GetPendingAndRetryTasks(limit int, userId int64, storeIds []int64) ([]api.ProductImportTaskRespDTO, error) {
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
			logrus.Warnf("[GetPendingAndRetryTasks] 未获取到任何任务！请求参数: limit=%d, userId=%d, storeIds=%v",
				limit, userId, storeIds)
		}
		return *tasksPtr, nil
	}

	return nil, fmt.Errorf("无法解析任务列表")
}

// UpdateTaskStatus 更新任务状态
func (m *ImportTaskAPIClient) UpdateTaskStatus(req *api.ProductImportTaskUpdateReqDTO) error {
	url := fmt.Sprintf("%s/rpc-api/listing/import-task/update-status", m.baseURL)

	var result APIResponse
	if err := m.apiRequest(http.MethodPost, url, req, &result); err != nil {
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return fmt.Errorf("处理API响应失败: %w", err)
	}

	return nil
}
