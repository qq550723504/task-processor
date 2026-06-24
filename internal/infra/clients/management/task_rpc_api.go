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
	localProvider *LocalTaskRPCProvider
}

// SubmitTask 提交单个任务。
func (c *TaskRPCAPIClient) SubmitTask(req *api.TaskSubmitReqDTO) (*api.TaskSubmitRespDTO, error) {
	if c.localProvider != nil {
		resp, handled, err := c.localProvider.SubmitTask(req, false)
		if err != nil {
			return nil, fmt.Errorf("提交任务失败: %w", err)
		}
		if handled {
			return resp, nil
		}
		return nil, fmt.Errorf("本地任务提交未处理")
	}
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
	if c.localProvider != nil {
		resp, handled, err := c.localProvider.SubmitBatchTasks(req)
		if err != nil {
			return nil, fmt.Errorf("批量提交任务失败: %w", err)
		}
		if handled {
			return resp, nil
		}
		return nil, fmt.Errorf("本地批量任务提交未处理")
	}
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
	if c.localProvider != nil {
		resp, handled, err := c.localProvider.SubmitTask(req, true)
		if err != nil {
			return nil, fmt.Errorf("提交紧急任务失败: %w", err)
		}
		if handled {
			return resp, nil
		}
		return nil, fmt.Errorf("本地紧急任务提交未处理")
	}
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
	if c.localProvider != nil {
		resp, handled, err := c.localProvider.GetTaskStatus(taskID)
		if err != nil {
			return nil, fmt.Errorf("查询任务状态失败: %w", err)
		}
		if handled {
			return resp, nil
		}
		return nil, nil
	}
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
	if c.localProvider != nil {
		resp, handled, err := c.localProvider.GetBatchTaskStatus(taskIDs)
		if err != nil {
			return nil, fmt.Errorf("批量查询任务状态失败: %w", err)
		}
		if handled {
			return resp, nil
		}
		return []api.TaskStatusRespDTO{}, nil
	}
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
	if c.localProvider != nil {
		resp, handled, err := c.localProvider.CancelTask(taskID)
		if err != nil {
			return nil, fmt.Errorf("取消任务失败: %w", err)
		}
		if handled {
			return resp, nil
		}
		return localTaskActionNotFound(taskID, "cancel"), nil
	}
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
	if c.localProvider != nil {
		resp, handled, err := c.localProvider.RetryTask(taskID)
		if err != nil {
			return nil, fmt.Errorf("重试任务失败: %w", err)
		}
		if handled {
			return resp, nil
		}
		return localTaskActionNotFound(taskID, "retry"), nil
	}
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
	if c.localProvider != nil {
		resp, handled, err := c.localProvider.GetQueueStats()
		if err != nil {
			return "", fmt.Errorf("获取队列统计失败: %w", err)
		}
		if handled {
			return resp, nil
		}
		return `{"source":"local-db","summary":{"pending":0,"processing":0,"total":0},"byStatus":{}}`, nil
	}
	url := fmt.Sprintf("%s/rpc-api/listing/task/queue-stats", c.baseURL)

	resp, err := getTypedResult[string](c.ManagementAPIClient, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("获取队列统计失败: %w", err)
	}
	return resp, nil
}

func localTaskActionNotFound(taskID int64, action string) *api.TaskActionRespDTO {
	return &api.TaskActionRespDTO{
		TaskID:       taskID,
		Action:       action,
		Success:      false,
		ErrorMessage: "本地任务不存在",
	}
}
