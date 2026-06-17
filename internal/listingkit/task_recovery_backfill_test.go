package listingkit

import (
	"context"
	"testing"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
)

func TestBackfillRetryableFailuresReclassifiesKnownCreditErrors(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryBackfillTestRepo()
	ctx := WithTenantID(context.Background(), "tenant-backfill")
	now := time.Date(2026, 6, 6, 18, 0, 0, 0, time.UTC)

	retryableTask := &Task{
		ID:        "task-backfill-retryable",
		TenantID:  "tenant-backfill",
		Status:    TaskStatusFailed,
		Request:   &GenerateRequest{TenantID: "tenant-backfill", Platforms: []string{"shein"}, Text: "retryable"},
		Error:     "openai request failed: insufficient credits",
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	}
	if err := repo.CreateTask(ctx, retryableTask); err != nil {
		t.Fatalf("CreateTask(retryable) error = %v", err)
	}

	nonRetryableTask := &Task{
		ID:        "task-backfill-non-retryable",
		TenantID:  "tenant-backfill",
		Status:    TaskStatusFailed,
		Request:   &GenerateRequest{TenantID: "tenant-backfill", Platforms: []string{"shein"}, Text: "non-retryable"},
		Error:     "validation failed: missing required category_id",
		CreatedAt: now.Add(-30 * time.Minute),
		UpdatedAt: now.Add(-30 * time.Minute),
	}
	if err := repo.CreateTask(ctx, nonRetryableTask); err != nil {
		t.Fatalf("CreateTask(non-retryable) error = %v", err)
	}

	count, err := backfillRetryableBlockedTasks(ctx, repo, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("backfillRetryableBlockedTasks() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("backfillRetryableBlockedTasks() count = %d, want 1", count)
	}

	storedRetryable, err := repo.GetTask(ctx, retryableTask.ID)
	if err != nil {
		t.Fatalf("GetTask(retryable) error = %v", err)
	}
	if storedRetryable.Status != TaskStatusBlockedRetryable {
		t.Fatalf("retryable status = %q, want %q", storedRetryable.Status, TaskStatusBlockedRetryable)
	}
	if storedRetryable.RetryableBlock == nil {
		t.Fatal("retryable RetryableBlock = nil, want backfilled retryable metadata")
	}
	if storedRetryable.RetryableBlock.ReasonCode != submissiondomain.RetryableReasonCodeOpenAIInsufficientCredits {
		t.Fatalf("retryable ReasonCode = %q, want %q", storedRetryable.RetryableBlock.ReasonCode, submissiondomain.RetryableReasonCodeOpenAIInsufficientCredits)
	}
	if storedRetryable.RetryableBlock.NextRetryAt == nil {
		t.Fatal("retryable NextRetryAt = nil, want scheduled retry metadata")
	}
	if !storedRetryable.RetryableBlock.AutoResumeEnabled {
		t.Fatal("retryable AutoResumeEnabled = false, want true")
	}

	storedNonRetryable, err := repo.GetTask(ctx, nonRetryableTask.ID)
	if err != nil {
		t.Fatalf("GetTask(non-retryable) error = %v", err)
	}
	if storedNonRetryable.Status != TaskStatusFailed {
		t.Fatalf("non-retryable status = %q, want %q", storedNonRetryable.Status, TaskStatusFailed)
	}
	if storedNonRetryable.RetryableBlock != nil {
		t.Fatalf("non-retryable RetryableBlock = %+v, want nil", storedNonRetryable.RetryableBlock)
	}
}

type taskRecoveryBackfillTestRepo struct {
	tasks map[string]*Task
}

func newTaskRecoveryBackfillTestRepo() *taskRecoveryBackfillTestRepo {
	return &taskRecoveryBackfillTestRepo{tasks: map[string]*Task{}}
}

func (r *taskRecoveryBackfillTestRepo) CreateTask(_ context.Context, task *Task) error {
	copied := *task
	copied.RetryableBlock = cloneRetryableBlock(task.RetryableBlock)
	r.tasks[task.ID] = &copied
	return nil
}

func (r *taskRecoveryBackfillTestRepo) GetTask(_ context.Context, taskID string) (*Task, error) {
	task, ok := r.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}
	copied := *task
	copied.RetryableBlock = cloneRetryableBlock(task.RetryableBlock)
	return &copied, nil
}

func (r *taskRecoveryBackfillTestRepo) ListTasks(_ context.Context, query *TaskListQuery) ([]Task, int64, error) {
	page := 1
	pageSize := 100
	if query != nil {
		if query.Page > 0 {
			page = query.Page
		}
		if query.PageSize > 0 {
			pageSize = query.PageSize
		}
	}

	items := make([]Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		if query != nil && query.Status != "" && string(task.Status) != query.Status {
			continue
		}
		copied := *task
		copied.RetryableBlock = cloneRetryableBlock(task.RetryableBlock)
		items = append(items, copied)
	}

	total := int64(len(items))
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []Task{}, total, nil
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], total, nil
}

func (r *taskRecoveryBackfillTestRepo) MarkProcessing(context.Context, string) error {
	return nil
}

func (r *taskRecoveryBackfillTestRepo) MarkCompleted(context.Context, string, *ListingKitResult) error {
	return nil
}

func (r *taskRecoveryBackfillTestRepo) MarkNeedsReview(context.Context, string, *ListingKitResult, string) error {
	return nil
}

func (r *taskRecoveryBackfillTestRepo) MarkFailed(_ context.Context, taskID string, errorMsg string) error {
	task, ok := r.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	task.Status = TaskStatusFailed
	task.Error = errorMsg
	return nil
}

func (r *taskRecoveryBackfillTestRepo) MarkBlockedRetryable(_ context.Context, taskID string, block *RetryableBlock, errorMsg string) error {
	task, ok := r.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	task.Status = TaskStatusBlockedRetryable
	task.Error = errorMsg
	task.RetryableBlock = cloneRetryableBlock(block)
	return nil
}

func (r *taskRecoveryBackfillTestRepo) ListRecoverableTasks(context.Context, *RecoverableTaskQuery) ([]Task, error) {
	return nil, nil
}

func (r *taskRecoveryBackfillTestRepo) RecoverBlockedTaskNow(context.Context, string, time.Time) error {
	return nil
}

func (r *taskRecoveryBackfillTestRepo) BulkRecoverBlockedTasks(context.Context, *RecoverBlockedTasksQuery) (int64, error) {
	return 0, nil
}

func (r *taskRecoveryBackfillTestRepo) PrepareRetry(context.Context, string) error {
	return nil
}

func (r *taskRecoveryBackfillTestRepo) IncrementRetryCount(context.Context, string) error {
	return nil
}

func (r *taskRecoveryBackfillTestRepo) SaveTaskResult(context.Context, string, *ListingKitResult) error {
	return nil
}
