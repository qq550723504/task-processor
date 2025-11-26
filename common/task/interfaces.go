package task

import "task-processor/common/processor"

// TaskSubmitter 任务提交器接口
type TaskSubmitter interface {
	// SubmitTask 提交任务到对应平台的工作池
	SubmitTask(taskData string) error
	// GetPlatform 获取平台类型
	GetPlatform() string
	// GetAvailableSlots 获取可用槽位数
	GetAvailableSlots() int
	// GetQueueStats 获取队列统计信息
	GetQueueStats() processor.QueueStats
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
	GetPendingAndRetryTasks(maxTasks int, userID int64, storeIDs []int64) ([]TaskDTO, error)
	// UpdateTaskStatus 更新任务状态
	UpdateTaskStatus(taskID int64, status int16, errorMessage string) error
}

// StoreClient 店铺API客户端接口
type StoreClient interface {
	// GetStore 获取店铺信息
	GetStore(storeID int64) (*StoreDTO, error)
}

// TaskDTO 任务数据传输对象
type TaskDTO struct {
	ID         int64  `json:"id"`
	TenantID   int64  `json:"tenantId"`
	ProductID  string `json:"productId"`
	Platform   string `json:"platform"`
	Region     string `json:"region"`
	StoreID    int64  `json:"storeId"`
	CategoryID int64  `json:"categoryId"`
	CreateTime int64  `json:"createTime"`
	RetryCount int    `json:"retryCount"`
	Priority   int    `json:"priority"`
	Creator    string `json:"creator"`
}

// StoreDTO 店铺数据传输对象
type StoreDTO struct {
	ID       int64  `json:"id"`
	Platform string `json:"platform"`
	Name     string `json:"name"`
}
