package task

import (
	"context"
	"task-processor/internal/infra/worker"
	"task-processor/internal/listingruntime"
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

type pendingRuntimeTaskSource interface {
	GetPendingRuntimeTasks(maxTasks int, userID int64, storeIDs []int64) ([]listingruntime.ImportTask, error)
}

type DailyListingCount struct {
	Count int64
}

type dailyListingCountReader interface {
	GetDailyListingCount(tenantID, storeID, userID int64, date string) (*DailyListingCount, error)
}

type runtimeTaskStatusUpdater interface {
	UpdateRuntimeTaskStatus(req *listingruntime.TaskStatusUpdate) error
}

type storeDispatchRuntime interface {
	StoreClient
	GetStorePauseStatus(storeID int64) (bool, error)
	GetStorePauseStatusDetail(storeID int64) (*listingruntime.StorePauseStatusDetail, error)
	SetStorePauseStatus(storeID int64, pause bool, pauseType string) (bool, error)
}

// StoreClient 店铺API客户端接口
type StoreClient interface {
	// GetStore 获取店铺信息
	GetStore(storeID int64) (*listingruntime.StoreInfo, error)
}
