package task

import (
	"context"
	"task-processor/internal/infra/worker"
	"task-processor/internal/pkg/management/api"
)

// TaskSubmitter 任务提交器接口
type TaskSubmitter interface {
	// SubmitTask 提交任务到对应平台的工作池
	SubmitTask(ctx context.Context, taskData string) error
	// GetPlatform 获取平台类型
	GetPlatform() string
	// GetAvailableSlots 获取可用槽位数
	GetAvailableSlots() int
	// GetQueueStats 获取队列统计信息
	GetQueueStats() worker.QueueStats
}

// ManagementClientProvider 管理客户端提供者接口
type ManagementClientProvider interface {
	// GetImportTaskClient 获取导入任务API客户端
	GetImportTaskClient() ImportTaskClient
	// GetStoreClient 获取店铺API客户端
	GetStoreClient() StoreClient
}

// ImportTaskClient 导入任务API客户端接口
type ImportTaskClient interface {
	// GetPendingAndRetryTasks 获取待处理和重试任务
	GetPendingAndRetryTasks(maxTasks int, userID int64, storeIDs []int64) ([]api.ProductImportTaskRespDTO, error)
	// UpdateTaskStatus 更新任务状态
	UpdateTaskStatus(taskID int64, status int16, errorMessage string) error
}

// StoreClient 店铺API客户端接口
type StoreClient interface {
	// GetStore 获取店铺信息
	GetStore(storeID int64) (*api.StoreRespDTO, error)
}
