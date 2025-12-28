package impl

import (
	"fmt"
	"net/http"
	"task-processor/internal/common/management/api"

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
		logrus.Debugf("[GetPendingAndRetryTasks] 添加 userId 过滤条件: %d", userId)
	} else {
		logrus.Debug("[GetPendingAndRetryTasks] 未指定 userId，将获取所有用户的任务")
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
		logrus.Debugf("[GetPendingAndRetryTasks] 添加 storeIds 过滤条件: %v", storeIds)
	} else {
		logrus.Debug("[GetPendingAndRetryTasks] 未指定 storeIds，将获取所有店铺的任务")
	}

	logrus.Infof("[GetPendingAndRetryTasks] 开始请求任务列表 - URL: %s, limit: %d, userId: %d, storeIds: %v",
		url, limit, userId, storeIds)

	var result APIResponse
	var tasks []api.ProductImportTaskRespDTO
	result.Data = &tasks

	// 使用GET请求
	logrus.Debug("[GetPendingAndRetryTasks] 发送 HTTP GET 请求...")
	err := m.apiRequest(http.MethodGet, url, nil, &result)
	if err != nil {
		logrus.Errorf("[GetPendingAndRetryTasks] HTTP 请求失败: %v", err)
		return nil, fmt.Errorf("请求导入任务失败: %w", err)
	}
	logrus.Debug("[GetPendingAndRetryTasks] HTTP 请求成功")

	// 记录原始响应信息
	logrus.Debugf("[GetPendingAndRetryTasks] API 响应码: %d, 消息: %s", result.Code, result.Message)

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		logrus.Errorf("[GetPendingAndRetryTasks] API 响应处理失败 - Code: %d, Message: %s, Error: %v",
			result.Code, result.Message, err)
		return nil, fmt.Errorf("处理API响应失败: %w", err)
	}
	logrus.Debug("[GetPendingAndRetryTasks] API 响应处理成功")

	// 安全的类型断言
	if tasksPtr, ok := result.Data.(*[]api.ProductImportTaskRespDTO); ok {
		taskCount := len(*tasksPtr)
		logrus.Infof("[GetPendingAndRetryTasks] ✓ 成功获取 %d 个待处理任务", taskCount)

		// 如果获取到的任务数为 0，记录详细信息
		if taskCount == 0 {
			logrus.Warnf("[GetPendingAndRetryTasks] ⚠ 未获取到任何任务！请求参数: limit=%d, userId=%d, storeIds=%v",
				limit, userId, storeIds)
		} else {
			// 记录前几个任务的基本信息（用于调试）
			logrus.Debug("[GetPendingAndRetryTasks] 任务列表详情:")
			for i, task := range *tasksPtr {
				if i < 3 { // 只记录前3个任务
					logrus.Debugf("  - 任务 #%d: ID=%d, Status=%d, StoreID=%d, Platform=%s, ProductID=%s, RetryCount=%d",
						i+1, task.ID, task.Status, task.StoreID, task.Platform, task.ProductID, task.RetryCount)
				}
			}
			if taskCount > 3 {
				logrus.Debugf("  ... 还有 %d 个任务未显示", taskCount-3)
			}
		}

		return *tasksPtr, nil
	}

	logrus.Error("[GetPendingAndRetryTasks] 类型断言失败，无法解析任务列表")
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

	// 注意：成功日志由上层TaskStatusUpdater统一记录，避免重复日志
	return nil
}
