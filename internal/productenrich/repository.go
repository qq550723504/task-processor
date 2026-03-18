// package productenrich 定义产品JSON生成的领域模型
package productenrich

import "context"

// TaskRepository 任务仓储接口（在 domain 层定义，在 infra 层实现）
type TaskRepository interface {
	CreateTask(ctx context.Context, task *Task) error
	GetTask(ctx context.Context, taskID string) (*Task, error)
	UpdateTaskStatus(ctx context.Context, taskID string, status TaskStatus) error
	UpdateTaskError(ctx context.Context, taskID string, errorMsg string) error
	SaveTaskResult(ctx context.Context, taskID string, result *ProductJSON) error
	IncrementRetryCount(ctx context.Context, taskID string) error
	// ResetForRetry 将任务状态重置为 pending，用于重试
	ResetForRetry(ctx context.Context, taskID string) error
}
