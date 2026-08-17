package listingkit

import (
	"context"
	"time"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	"task-processor/internal/listingkit/core"
)

type stubServiceDeferredRenderer struct {
	result *asset.AssetRecord
}

type stubGenerationRepo struct {
	task                   *Task
	generationUsageRenewed chan struct{}
	markGenerationUsageErr error
}

func (r *stubGenerationRepo) CreateTask(ctx context.Context, task *Task) error {
	copied := *task
	r.task = &copied
	return nil
}

func (r *stubGenerationRepo) GetTask(ctx context.Context, taskID string) (*Task, error) {
	if r.task == nil || r.task.ID != taskID {
		return nil, core.ErrTaskNotFound
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
	r.task.Status = core.TaskStatusNeedsReview
	r.task.RetryableBlock = nil
	r.task.Error = reason
	return nil
}
func (r *stubGenerationRepo) MarkFailed(ctx context.Context, taskID string, errorMsg string) error {
	return nil
}
func (r *stubGenerationRepo) MarkBlockedRetryable(ctx context.Context, taskID string, block *RetryableBlock, errorMsg string) error {
	if r.task == nil || r.task.ID != taskID {
		return core.ErrTaskNotFound
	}
	r.task.Status = core.TaskStatusBlockedRetryable
	r.task.RetryableBlock = block
	r.task.Error = errorMsg
	r.task.UpdatedAt = time.Now()
	return nil
}
func (r *stubGenerationRepo) ListRecoverableTasks(context.Context, *RecoverableTaskQuery) ([]Task, error) {
	return []Task{}, nil
}
func (r *stubGenerationRepo) RecoverBlockedTaskNow(_ context.Context, taskID string, recoveredAt time.Time) error {
	if r.task == nil || r.task.ID != taskID {
		return core.ErrTaskNotFound
	}
	r.task.Status = core.TaskStatusPending
	r.task.RetryableBlock = nil
	r.task.Error = ""
	r.task.UpdatedAt = recoveredAt
	return nil
}
func (r *stubGenerationRepo) BulkRecoverBlockedTasks(context.Context, *RecoverBlockedTasksQuery) (int64, error) {
	return 0, nil
}
func (r *stubGenerationRepo) PrepareRetry(ctx context.Context, taskID string) error { return nil }
func (r *stubGenerationRepo) IncrementRetryCount(ctx context.Context, taskID string) error {
	return nil
}
func (r *stubGenerationRepo) SaveTaskResult(ctx context.Context, taskID string, result *ListingKitResult) error {
	if r.task == nil || r.task.ID != taskID {
		return core.ErrTaskNotFound
	}
	r.task.Result = result
	r.task.UpdatedAt = time.Now()
	return nil
}

func (r *stubGenerationRepo) BeginGenerationUsageReservation(_ context.Context, taskID string, leaseUntil time.Time) error {
	if r.task == nil || r.task.ID != taskID {
		return core.ErrTaskNotFound
	}
	if r.task.GenerationUsageReservationState == "" {
		r.task.GenerationUsageReservationState = GenerationUsageReservationStatePending
	}
	r.task.GenerationUsageReservationLeaseUntil = &leaseUntil
	return nil
}

func (r *stubGenerationRepo) MarkGenerationUsageReserved(_ context.Context, taskID string, leaseUntil time.Time) error {
	if r.markGenerationUsageErr != nil {
		return r.markGenerationUsageErr
	}
	if r.task == nil || r.task.ID != taskID || r.task.GenerationUsageReservationState == "" {
		return core.ErrTaskNotRecoverable
	}
	r.task.GenerationUsageReservationState = GenerationUsageReservationStateReserved
	r.task.GenerationUsageReservationLeaseUntil = &leaseUntil
	return nil
}

func (r *stubGenerationRepo) RenewGenerationUsageReservation(_ context.Context, taskID string, leaseUntil time.Time) error {
	if r.task == nil || r.task.ID != taskID || r.task.GenerationUsageReservationState == "" {
		return core.ErrTaskNotRecoverable
	}
	r.task.GenerationUsageReservationLeaseUntil = &leaseUntil
	if r.generationUsageRenewed != nil {
		select {
		case r.generationUsageRenewed <- struct{}{}:
		default:
		}
	}
	return nil
}

func (r *stubGenerationRepo) ClearGenerationUsageReservation(_ context.Context, taskID string) error {
	if r.task == nil || r.task.ID != taskID {
		return core.ErrTaskNotFound
	}
	r.task.GenerationUsageReservationState = ""
	r.task.GenerationUsageReservationLeaseUntil = nil
	return nil
}

func (r *stubGenerationRepo) PrepareGenerationUsageRelease(_ context.Context, taskID string, block *RetryableBlock, errorMsg string, result *ListingKitResult) error {
	if r.task == nil || r.task.ID != taskID || block == nil || block.ReasonCode != usageReleasePendingReason {
		return core.ErrTaskNotRecoverable
	}
	r.task.Status = core.TaskStatusBlockedRetryable
	r.task.RetryableBlock = cloneRetryableBlock(block)
	r.task.Error = errorMsg
	if result != nil {
		r.task.Result = result
	}
	return nil
}

func (r *stubGenerationRepo) ResolveGenerationUsageRelease(_ context.Context, taskID, terminalError string) error {
	if r.task == nil || r.task.ID != taskID || r.task.RetryableBlock == nil || r.task.RetryableBlock.ReasonCode != usageReleasePendingReason {
		return core.ErrTaskNotRecoverable
	}
	r.task.Status = core.TaskStatusFailed
	r.task.RetryableBlock = nil
	r.task.Error = terminalError
	r.task.GenerationUsageReservationState = ""
	r.task.GenerationUsageReservationLeaseUntil = nil
	return nil
}

func (r *stubGenerationRepo) ListExpiredGenerationUsageReservations(_ context.Context, dueBefore time.Time, limit int) ([]Task, error) {
	if r.task == nil || r.task.GenerationUsageReservationState == "" || r.task.GenerationUsageReservationLeaseUntil == nil || r.task.GenerationUsageReservationLeaseUntil.After(dueBefore) {
		return nil, nil
	}
	if limit == 0 {
		return nil, nil
	}
	copied := *r.task
	return []Task{copied}, nil
}

func (r *stubGenerationRepo) ResolveExpiredGenerationUsageReservation(_ context.Context, taskID string, expectedStatus core.TaskStatus, dueBefore time.Time, block *RetryableBlock, errorMsg string, clearReservation bool) error {
	if r.task == nil || r.task.ID != taskID || r.task.Status != expectedStatus || r.task.GenerationUsageReservationState == "" || r.task.GenerationUsageReservationLeaseUntil == nil || r.task.GenerationUsageReservationLeaseUntil.After(dueBefore) {
		return core.ErrTaskNotRecoverable
	}
	r.task.Status = core.TaskStatusBlockedRetryable
	r.task.RetryableBlock = block
	r.task.Error = errorMsg
	if clearReservation {
		r.task.GenerationUsageReservationState = ""
		r.task.GenerationUsageReservationLeaseUntil = nil
	}
	return nil
}

func (s *stubServiceDeferredRenderer) Render(ctx context.Context, req assetgeneration.DeferredRenderRequest) (*asset.AssetRecord, error) {
	return s.result, nil
}
