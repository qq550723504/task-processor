package taskrpcapi

import "task-processor/internal/pkg/types"

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
