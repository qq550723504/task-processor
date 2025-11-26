package impl

import (
	"fmt"
	"net/http"
	"task-processor/common/management/api"

	"github.com/sirupsen/logrus"
)

// ImportTaskAPIClientImpl 导入任务API客户端实现
type ImportTaskAPIClientImpl struct {
	*ManagementAPIClientImpl
}

// GetPendingAndRetryTasks 获取待处理及待重试的任务列表
func (m *ImportTaskAPIClientImpl) GetPendingAndRetryTasks(limit int, userId int64, storeIds []int64) ([]api.ProductImportTaskRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/import-task/list-pending-and-retry?limit=%d", m.baseURL, limit)

	// 添加可选参数
	if userId > 0 {
		url = fmt.Sprintf("%s&userId=%d", url, userId)
	}

	// 支持多个店铺编号，使用逗号分割
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

	logrus.Infof("正在请求导入任务API: %s", url)

	var result APIResponse
	var tasks []api.ProductImportTaskRespDTO
	result.Data = &tasks

	// 使用GET请求
	err := m.apiRequest(http.MethodGet, url, nil, &result)
	if err != nil {
		return nil, fmt.Errorf("请求导入任务失败: %w", err)
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, fmt.Errorf("处理API响应失败: %w", err)
	}

	// 安全的类型断言
	if tasksPtr, ok := result.Data.(*[]api.ProductImportTaskRespDTO); ok {
		logrus.Infof("成功获取 %d 个待处理任务", len(*tasksPtr))
		return *tasksPtr, nil
	}

	return nil, fmt.Errorf("无法解析任务列表")
}

// UpdateTaskStatus 更新任务状态
func (m *ImportTaskAPIClientImpl) UpdateTaskStatus(req *api.ProductImportTaskUpdateReqDTO) error {
	url := fmt.Sprintf("%s/rpc-api/listing/import-task/update-status", m.baseURL)

	var result APIResponse

	// 使用POST请求
	err := m.apiRequest(http.MethodPost, url, req, &result)
	if err != nil {
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return fmt.Errorf("处理API响应失败: %w", err)
	}

	logrus.Infof("成功更新任务状态: TaskID=%d, Status=%d", req.ID, req.Status)
	return nil
}
