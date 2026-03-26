package productenrich

import "context"

// TaskRepository defines task persistence operations used by the productenrich flow.
type TaskRepository interface {
	CreateTask(ctx context.Context, task *Task) error
	GetTask(ctx context.Context, taskID string) (*Task, error)

	// High-level lifecycle operations.
	MarkProcessing(ctx context.Context, taskID string) error
	MarkCompleted(ctx context.Context, taskID string, result *ProductJSON) error
	MarkFailed(ctx context.Context, taskID string, errorMsg string) error
	PrepareRetry(ctx context.Context, taskID string) error

	// Low-level operations kept for compatibility with existing callers/tests.
	UpdateTaskStatus(ctx context.Context, taskID string, status TaskStatus) error
	UpdateTaskError(ctx context.Context, taskID string, errorMsg string) error
	SaveTaskResult(ctx context.Context, taskID string, result *ProductJSON) error
	IncrementRetryCount(ctx context.Context, taskID string) error
	ResetForRetry(ctx context.Context, taskID string) error
}
