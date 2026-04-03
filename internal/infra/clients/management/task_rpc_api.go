package management

import (
	"fmt"
	"net/http"
	"task-processor/internal/core/logger"
	"task-processor/internal/infra/clients/management/api"
)

// TaskRPCAPIClient 任务 RPC 客户端。
type TaskRPCAPIClient struct {
	*ManagementAPIClient
}

// SubmitTask 提交单个任务。
func (c *TaskRPCAPIClient) SubmitTask(req *api.TaskSubmitReqDTO) (*api.TaskSubmitRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/task/submit", c.baseURL)

	resp, err := getTypedResult[api.TaskSubmitRespDTO](c.ManagementAPIClient, http.MethodPost, url, req)
	if err != nil {
		return nil, fmt.Errorf("提交任务失败: %w", err)
	}

	logger.GetGlobalLogger("infra/clients").Debugf(
		"[TaskRPC] submit taskId=%d success=%v statusKey=%s canonicalStatus=%s queue=%s",
		resp.TaskID,
		resp.Success,
		resp.StatusKey,
		resp.CanonicalStatus,
		resp.QueueName,
	)
	return &resp, nil
}

// SubmitBatchTasks 批量提交任务。
func (c *TaskRPCAPIClient) SubmitBatchTasks(req *api.TaskBatchSubmitReqDTO) (*api.TaskBatchSubmitRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/task/submit-batch", c.baseURL)

	resp, err := getTypedResult[api.TaskBatchSubmitRespDTO](c.ManagementAPIClient, http.MethodPost, url, req)
	if err != nil {
		return nil, fmt.Errorf("批量提交任务失败: %w", err)
	}

	logger.GetGlobalLogger("infra/clients").Debugf(
		"[TaskRPC] submit-batch batchId=%s total=%d success=%d failure=%d statusKey=%s canonicalStatus=%s",
		resp.BatchID,
		resp.TotalCount,
		resp.SuccessCount,
		resp.FailureCount,
		resp.StatusKey,
		resp.CanonicalStatus,
	)
	return &resp, nil
}

// SubmitUrgentTask 提交紧急任务。
func (c *TaskRPCAPIClient) SubmitUrgentTask(req *api.TaskSubmitReqDTO) (*api.TaskSubmitRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/task/submit-urgent", c.baseURL)

	resp, err := getTypedResult[api.TaskSubmitRespDTO](c.ManagementAPIClient, http.MethodPost, url, req)
	if err != nil {
		return nil, fmt.Errorf("提交紧急任务失败: %w", err)
	}

	logger.GetGlobalLogger("infra/clients").Debugf(
		"[TaskRPC] submit-urgent taskId=%d success=%v statusKey=%s canonicalStatus=%s queue=%s",
		resp.TaskID,
		resp.Success,
		resp.StatusKey,
		resp.CanonicalStatus,
		resp.QueueName,
	)
	return &resp, nil
}

// GetTaskStatus 查询单个任务状态。
func (c *TaskRPCAPIClient) GetTaskStatus(taskID int64) (*api.TaskStatusRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/task/status", c.baseURL)
	req := &api.TaskStatusReqDTO{TaskID: taskID}

	resp, err := getTypedResult[api.TaskStatusRespDTO](c.ManagementAPIClient, http.MethodPost, url, req)
	if err != nil {
		return nil, fmt.Errorf("查询任务状态失败: %w", err)
	}

	logger.GetGlobalLogger("infra/clients").Debugf(
		"[TaskRPC] status taskId=%d status=%s canonicalStatus=%s reason=%s",
		resp.TaskID,
		resp.StatusKey,
		resp.CanonicalStatus,
		resp.ErrorMessage,
	)
	return &resp, nil
}

// GetBatchTaskStatus 批量查询任务状态。
func (c *TaskRPCAPIClient) GetBatchTaskStatus(taskIDs []int64) ([]api.TaskStatusRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/task/status-batch", c.baseURL)

	var result APIResponse
	statuses := &[]api.TaskStatusRespDTO{}
	result.Data = statuses
	if err := c.apiRequest(http.MethodPost, url, taskIDs, &result); err != nil {
		return nil, fmt.Errorf("批量查询任务状态失败: %w", err)
	}
	if err := c.ProcessAPIResponse(&result, 0); err != nil {
		return nil, fmt.Errorf("处理批量任务状态响应失败: %w", err)
	}

	resp, ok := result.Data.(*[]api.TaskStatusRespDTO)
	if !ok {
		return nil, fmt.Errorf("批量任务状态响应数据类型转换失败")
	}

	logger.GetGlobalLogger("infra/clients").Debugf("[TaskRPC] status-batch count=%d", len(*resp))
	return *resp, nil
}

// CancelTask 取消任务。
func (c *TaskRPCAPIClient) CancelTask(taskID int64) (*api.TaskActionRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/task/cancel/%d", c.baseURL, taskID)

	resp, err := getTypedResult[api.TaskActionRespDTO](c.ManagementAPIClient, http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("取消任务失败: %w", err)
	}

	logger.GetGlobalLogger("infra/clients").Debugf(
		"[TaskRPC] cancel taskId=%d success=%v statusKey=%s canonicalStatus=%s",
		resp.TaskID,
		resp.Success,
		resp.StatusKey,
		resp.CanonicalStatus,
	)
	return &resp, nil
}

// RetryTask 重试任务。
func (c *TaskRPCAPIClient) RetryTask(taskID int64) (*api.TaskActionRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/task/retry/%d", c.baseURL, taskID)

	resp, err := getTypedResult[api.TaskActionRespDTO](c.ManagementAPIClient, http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("重试任务失败: %w", err)
	}

	logger.GetGlobalLogger("infra/clients").Debugf(
		"[TaskRPC] retry taskId=%d success=%v statusKey=%s canonicalStatus=%s",
		resp.TaskID,
		resp.Success,
		resp.StatusKey,
		resp.CanonicalStatus,
	)
	return &resp, nil
}

// GetQueueStats 获取队列统计。
func (c *TaskRPCAPIClient) GetQueueStats() (string, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/task/queue-stats", c.baseURL)

	resp, err := getTypedResult[string](c.ManagementAPIClient, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("获取队列统计失败: %w", err)
	}
	return resp, nil
}
