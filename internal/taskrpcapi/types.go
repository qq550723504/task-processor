package taskrpcapi

import "task-processor/internal/pkg/types"

// TaskSubmitReqDTO is the local task RPC submit request contract.
type TaskSubmitReqDTO struct {
	TaskID           int64  `json:"taskId"`
	TenantID         int64  `json:"tenantId"`
	StoreID          int64  `json:"storeId"`
	Platform         string `json:"platform"`
	TargetPlatform   string `json:"targetPlatform,omitempty"`
	SourcePlatform   string `json:"sourcePlatform,omitempty"`
	Region           string `json:"region"`
	CategoryID       int64  `json:"categoryId"`
	ProductID        string `json:"productId"`
	BusinessPriority int    `json:"businessPriority"`
	TaskType         string `json:"taskType"`
	TaskData         string `json:"taskData"`
	Description      string `json:"description,omitempty"`
	CallbackURL      string `json:"callbackUrl,omitempty"`
	TimeoutMinutes   int    `json:"timeoutMinutes,omitempty"`
	MaxRetries       int    `json:"maxRetries,omitempty"`
}

// TaskBatchSubmitReqDTO is the local task RPC batch submit request contract.
type TaskBatchSubmitReqDTO struct {
	BatchID         string             `json:"batchId,omitempty"`
	FailureStrategy string             `json:"failureStrategy,omitempty"`
	Tasks           []TaskSubmitReqDTO `json:"tasks"`
}

// TaskSubmitRespDTO is the local task RPC submit response contract.
type TaskSubmitRespDTO struct {
	TaskID             int64  `json:"taskId"`
	Success            bool   `json:"success"`
	MessagePriority    int    `json:"messagePriority"`
	RoutingKey         string `json:"routingKey"`
	QueueName          string `json:"queueName"`
	SubmitTime         string `json:"submitTime"`
	EstimatedStartTime string `json:"estimatedStartTime"`
	ErrorMessage       string `json:"errorMessage"`
	StatusKey          string `json:"statusKey"`
	StatusName         string `json:"statusName"`
	CanonicalStatus    string `json:"canonicalStatus"`
}

// TaskBatchSubmitRespDTO is the local task RPC batch submit response contract.
type TaskBatchSubmitRespDTO struct {
	BatchID          string              `json:"batchId"`
	TotalCount       int                 `json:"totalCount"`
	SuccessCount     int                 `json:"successCount"`
	FailureCount     int                 `json:"failureCount"`
	SubmitTime       string              `json:"submitTime"`
	ProcessingTimeMs int64               `json:"processingTimeMs"`
	SuccessTasks     []TaskSubmitRespDTO `json:"successTasks"`
	FailureTasks     []TaskSubmitRespDTO `json:"failureTasks"`
	Status           string              `json:"status"`
	ErrorMessage     string              `json:"errorMessage"`
	StatusKey        string              `json:"statusKey"`
	StatusName       string              `json:"statusName"`
	CanonicalStatus  string              `json:"canonicalStatus"`
}

// TaskStatusReqDTO is the local task RPC status request contract.
type TaskStatusReqDTO struct {
	TaskID         int64 `json:"taskId"`
	TenantID       int64 `json:"tenantId,omitempty"`
	IncludeDetails bool  `json:"includeDetails,omitempty"`
	IncludeLogs    bool  `json:"includeLogs,omitempty"`
}

// TaskStatusRespDTO is the local task RPC status response contract exposed by this HTTP module.
type TaskStatusRespDTO struct {
	TaskID           int64               `json:"taskId"`
	Status           string              `json:"status"`
	StatusKey        string              `json:"statusKey"`
	StatusName       string              `json:"statusName"`
	CanonicalStatus  string              `json:"canonicalStatus"`
	Platform         string              `json:"platform"`
	Region           string              `json:"region"`
	TaskType         string              `json:"taskType"`
	Priority         int                 `json:"priority"`
	RetryCount       int                 `json:"retryCount"`
	MaxRetries       int                 `json:"maxRetries"`
	CreatedAt        *types.FlexibleTime `json:"createdAt"`
	StartedAt        *types.FlexibleTime `json:"startedAt"`
	CompletedAt      *types.FlexibleTime `json:"completedAt"`
	ProcessingTimeMs int64               `json:"processingTimeMs"`
	QueueName        string              `json:"queueName"`
	ProcessingNode   string              `json:"processingNode"`
	ProgressPercent  int                 `json:"progressPercent"`
	Result           string              `json:"result"`
	ErrorMessage     string              `json:"errorMessage"`
	ErrorStack       string              `json:"errorStack"`
	ExecutionLogs    []string            `json:"executionLogs"`
	NextRetryAt      *types.FlexibleTime `json:"nextRetryAt"`
	TaskDetails      string              `json:"taskDetails"`
}

// TaskActionRespDTO is the local task RPC action response contract exposed by this HTTP module.
type TaskActionRespDTO struct {
	TaskID          int64  `json:"taskId"`
	Action          string `json:"action"`
	Success         bool   `json:"success"`
	StatusKey       string `json:"statusKey"`
	StatusName      string `json:"statusName"`
	CanonicalStatus string `json:"canonicalStatus"`
	ErrorMessage    string `json:"errorMessage"`
	ActionTime      string `json:"actionTime"`
}

// TaskRPCAPI is the local task RPC client interface.
type TaskRPCAPI interface {
	SubmitTask(req *TaskSubmitReqDTO) (*TaskSubmitRespDTO, error)
	SubmitBatchTasks(req *TaskBatchSubmitReqDTO) (*TaskBatchSubmitRespDTO, error)
	SubmitUrgentTask(req *TaskSubmitReqDTO) (*TaskSubmitRespDTO, error)
	GetTaskStatus(taskID int64) (*TaskStatusRespDTO, error)
	GetBatchTaskStatus(taskIDs []int64) ([]TaskStatusRespDTO, error)
	CancelTask(taskID int64) (*TaskActionRespDTO, error)
	RetryTask(taskID int64) (*TaskActionRespDTO, error)
	GetQueueStats() (string, error)
}
