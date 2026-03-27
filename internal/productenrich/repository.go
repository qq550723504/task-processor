package productenrich

import "context"

// TaskRepository 定义 productenrich 流程中使用的任务持久化操作。
type TaskRepository interface {
	CreateTask(ctx context.Context, task *Task) error
	GetTask(ctx context.Context, taskID string) (*Task, error)

	// 高级生命周期操作。
	MarkProcessing(ctx context.Context, taskID string) error
	MarkCompleted(ctx context.Context, taskID string, result *ProductJSON) error
	MarkFailed(ctx context.Context, taskID string, errorMsg string) error
	PrepareRetry(ctx context.Context, taskID string) error

	// 低级操作保留以与现有调用者/测试保持兼容。
	UpdateTaskStatus(ctx context.Context, taskID string, status TaskStatus) error
	UpdateTaskError(ctx context.Context, taskID string, errorMsg string) error
	SaveTaskResult(ctx context.Context, taskID string, result *ProductJSON) error
	IncrementRetryCount(ctx context.Context, taskID string) error
	ResetForRetry(ctx context.Context, taskID string) error
}
