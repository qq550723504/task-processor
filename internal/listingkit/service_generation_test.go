package listingkit

import (
	"context"
	"time"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
)

type stubServiceDeferredRenderer struct {
	result *asset.AssetRecord
}

type stubGenerationRepo struct {
	task *Task
}

func (r *stubGenerationRepo) CreateTask(ctx context.Context, task *Task) error {
	copied := *task
	r.task = &copied
	return nil
}

func (r *stubGenerationRepo) GetTask(ctx context.Context, taskID string) (*Task, error) {
	if r.task == nil || r.task.ID != taskID {
		return nil, ErrTaskNotFound
	}
	copied := *r.task
	return &copied, nil
}

func (r *stubGenerationRepo) ListTasks(ctx context.Context, query *TaskListQuery) ([]Task, int64, error) {
	if r.task == nil {
		return []Task{}, 0, nil
	}
	copied := *r.task
	return []Task{copied}, 1, nil
}

func (r *stubGenerationRepo) MarkProcessing(ctx context.Context, taskID string) error { return nil }
func (r *stubGenerationRepo) MarkCompleted(ctx context.Context, taskID string, result *ListingKitResult) error {
	return r.SaveTaskResult(ctx, taskID, result)
}
func (r *stubGenerationRepo) MarkNeedsReview(ctx context.Context, taskID string, result *ListingKitResult, reason string) error {
	if err := r.SaveTaskResult(ctx, taskID, result); err != nil {
		return err
	}
	r.task.Status = TaskStatusNeedsReview
	r.task.Error = reason
	return nil
}
func (r *stubGenerationRepo) MarkFailed(ctx context.Context, taskID string, errorMsg string) error {
	return nil
}
func (r *stubGenerationRepo) PrepareRetry(ctx context.Context, taskID string) error { return nil }
func (r *stubGenerationRepo) IncrementRetryCount(ctx context.Context, taskID string) error {
	return nil
}
func (r *stubGenerationRepo) SaveTaskResult(ctx context.Context, taskID string, result *ListingKitResult) error {
	if r.task == nil || r.task.ID != taskID {
		return ErrTaskNotFound
	}
	r.task.Result = result
	r.task.UpdatedAt = time.Now()
	return nil
}

func (s *stubServiceDeferredRenderer) Render(ctx context.Context, req assetgeneration.DeferredRenderRequest) (*asset.AssetRecord, error) {
	return s.result, nil
}
