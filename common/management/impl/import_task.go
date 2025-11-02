package impl

import (
	"encoding/json"
	"fmt"
	"task-processor/common/management/api"

	"github.com/sirupsen/logrus"
)

// ImportTaskClient 导入任务API客户端实现
type ImportTaskClient struct {
	*BaseManagementClient
}

// NewImportTaskClient 创建导入任务API客户端
func NewImportTaskClient(baseURL string) *ImportTaskClient {
	return &ImportTaskClient{
		BaseManagementClient: NewBaseManagementClient(baseURL),
	}
}

// GetPendingAndRetryTasks 获取待处理及待重试的任务列表
func (c *ImportTaskClient) GetPendingAndRetryTasks(limit int, userId int64, storeIds []int64, platform string) ([]api.ProductImportTaskRespDTO, error) {
	// 构建URL参数
	url := fmt.Sprintf("/rpc-api/listing/import-task/list-pending-and-retry?limit=%d", limit)

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

	// 添加平台参数
	if platform != "" {
		url = fmt.Sprintf("%s&platform=%s", url, platform)
	}

	logrus.Infof("正在请求导入任务API: %s", url)

	// 使用GET请求
	body, err := c.makeAPIRequestWithURL("GET", url)
	if err != nil {
		return nil, fmt.Errorf("请求导入任务失败: %w", err)
	}

	var result APIResponse
	var tasks []api.ProductImportTaskRespDTO
	result.Data = &tasks

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if err := c.ProcessAPIResponse(&result, 0); err != nil {
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
func (c *ImportTaskClient) UpdateTaskStatus(req *api.ProductImportTaskUpdateReqDTO) error {
	// 构建请求参数
	params := map[string]any{
		"id":     req.ID,
		"status": req.Status,
	}

	if req.ErrorMessage != "" {
		params["errorMessage"] = req.ErrorMessage
	}

	// 使用POST请求
	body, err := c.makeAPIRequest("POST", "/rpc-api/listing/import-task/update-status", params)
	if err != nil {
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	var result APIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		// 如果解析失败，但HTTP状态码是200，认为成功
		logrus.Debugf("状态更新响应解析失败，但HTTP状态正常: %v", err)
		return nil
	}

	if err := c.ProcessAPIResponse(&result, 0); err != nil {
		return fmt.Errorf("处理API响应失败: %w", err)
	}

	logrus.Infof("成功更新任务状态: TaskID=%d, Status=%d", req.ID, req.Status)
	return nil
}
