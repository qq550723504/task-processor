package productimage

import "context"

type TaskRepository interface {
	CreateTask(ctx context.Context, task *Task) error
	GetTask(ctx context.Context, taskID string) (*Task, error)

	MarkProcessing(ctx context.Context, taskID string) error
	MarkCompleted(ctx context.Context, taskID string, result *ImageProcessResult) error
	MarkNeedsReview(ctx context.Context, taskID string, result *ImageProcessResult, reason string) error
	MarkRejected(ctx context.Context, taskID string, reason string) error
	MarkFailed(ctx context.Context, taskID string, errorMsg string) error
	PrepareRetry(ctx context.Context, taskID string) error

	UpdateTaskStatus(ctx context.Context, taskID string, status TaskStatus) error
	UpdateTaskError(ctx context.Context, taskID string, errorMsg string) error
	SaveTaskResult(ctx context.Context, taskID string, result *ImageProcessResult) error
	IncrementRetryCount(ctx context.Context, taskID string) error
	ResetForRetry(ctx context.Context, taskID string) error
}
