package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

	listingsubmission "task-processor/internal/listing/submission"
)

type taskRecoveryTestSubmitter func(taskID string) error

func (f taskRecoveryTestSubmitter) Submit(taskID string) error { return f(taskID) }

func TestRecoverTaskNowRequeuesBlockedTask(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	ctx := WithTenantID(context.Background(), "tenant-recovery")
	now := time.Date(2026, 6, 6, 14, 0, 0, 0, time.UTC)
	nextRetryAt := now.Add(-time.Minute)
	task := &Task{
		ID:        "task-recover-now",
		TenantID:  "tenant-recovery",
		Status:    TaskStatusPending,
		Request:   &GenerateRequest{TenantID: "tenant-recovery", Platforms: []string{"amazon"}, Text: "recover now"},
		CreatedAt: now.Add(-10 * time.Minute),
		UpdatedAt: now.Add(-10 * time.Minute),
	}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := repo.MarkBlockedRetryable(ctx, task.ID, &RetryableBlock{
		ReasonCode:           listingsubmission.RetryableReasonCodeWorkerQueueBackpressure,
		ReasonMessage:        "queue full",
		BlockedAt:            now.Add(-5 * time.Minute),
		NextRetryAt:          &nextRetryAt,
		RetryAttempts:        2,
		MaxAutoRetryAttempts: 8,
		RecoveryScope:        listingsubmission.RetryableRecoveryScopeTask,
		AutoResumeEnabled:    true,
	}, "queue full"); err != nil {
		t.Fatalf("MarkBlockedRetryable() error = %v", err)
	}

	submitted := make([]string, 0, 1)
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo: repo,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(taskID string) error {
				submitted = append(submitted, taskID)
				return nil
			})
		},
		now: func() time.Time { return now },
	})

	recovered, err := svc.RecoverTaskNow(ctx, task.ID)
	if err != nil {
		t.Fatalf("RecoverTaskNow() error = %v", err)
	}
	if recovered.Status != TaskStatusPending {
		t.Fatalf("status = %q, want %q", recovered.Status, TaskStatusPending)
	}
	if recovered.RetryableBlock == nil || recovered.RetryableBlock.LastRetryAt == nil {
		t.Fatalf("LastRetryAt = %+v, want non-nil", recovered.RetryableBlock)
	}
	if len(submitted) != 1 || submitted[0] != task.ID {
		t.Fatalf("submitted = %v, want [%s]", submitted, task.ID)
	}
}

func TestRecoverTaskNowForcesImmediateRecoveryBeforeNextRetryAt(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	ctx := WithTenantID(context.Background(), "tenant-force-recovery")
	now := time.Date(2026, 6, 6, 14, 30, 0, 0, time.UTC)
	futureRetryAt := now.Add(30 * time.Minute)
	task := &Task{
		ID:        "task-force-recover-now",
		TenantID:  "tenant-force-recovery",
		Status:    TaskStatusPending,
		Request:   &GenerateRequest{TenantID: "tenant-force-recovery", Platforms: []string{"amazon"}, Text: "force recover now"},
		CreatedAt: now.Add(-10 * time.Minute),
		UpdatedAt: now.Add(-10 * time.Minute),
	}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := repo.MarkBlockedRetryable(ctx, task.ID, &RetryableBlock{
		ReasonCode:           listingsubmission.RetryableReasonCodeOpenAIRateLimited,
		ReasonMessage:        "rate limited",
		BlockedAt:            now.Add(-5 * time.Minute),
		NextRetryAt:          &futureRetryAt,
		RetryAttempts:        2,
		MaxAutoRetryAttempts: 8,
		RecoveryScope:        listingsubmission.RetryableRecoveryScopeTask,
		AutoResumeEnabled:    true,
	}, "rate limited"); err != nil {
		t.Fatalf("MarkBlockedRetryable() error = %v", err)
	}

	submitted := make([]string, 0, 1)
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo: repo,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(taskID string) error {
				submitted = append(submitted, taskID)
				return nil
			})
		},
		now: func() time.Time { return now },
	})

	recovered, err := svc.RecoverTaskNow(ctx, task.ID)
	if err != nil {
		t.Fatalf("RecoverTaskNow() error = %v", err)
	}
	if recovered.Status != TaskStatusPending {
		t.Fatalf("status = %q, want %q", recovered.Status, TaskStatusPending)
	}
	if recovered.RetryableBlock == nil || recovered.RetryableBlock.LastRetryAt == nil {
		t.Fatalf("LastRetryAt = %+v, want non-nil", recovered.RetryableBlock)
	}
	if len(submitted) != 1 || submitted[0] != task.ID {
		t.Fatalf("submitted = %v, want [%s]", submitted, task.ID)
	}
}

func TestRunRecoverySweepRequeuesDueBlockedTasksOnly(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	ctx := WithTenantID(context.Background(), "tenant-sweep")
	now := time.Date(2026, 6, 6, 15, 0, 0, 0, time.UTC)
	dueRetryAt := now.Add(-time.Minute)
	futureRetryAt := now.Add(10 * time.Minute)

	for _, fixture := range []struct {
		id          string
		nextRetryAt time.Time
	}{
		{id: "task-due", nextRetryAt: dueRetryAt},
		{id: "task-future", nextRetryAt: futureRetryAt},
	} {
		task := &Task{
			ID:        fixture.id,
			TenantID:  "tenant-sweep",
			Status:    TaskStatusPending,
			Request:   &GenerateRequest{TenantID: "tenant-sweep", Platforms: []string{"amazon"}, Text: fixture.id},
			CreatedAt: now.Add(-10 * time.Minute),
			UpdatedAt: now.Add(-10 * time.Minute),
		}
		if err := repo.CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask(%s) error = %v", fixture.id, err)
		}
		if err := repo.MarkBlockedRetryable(ctx, task.ID, &RetryableBlock{
			ReasonCode:           listingsubmission.RetryableReasonCodeWorkerQueueBackpressure,
			ReasonMessage:        "queue full",
			BlockedAt:            now.Add(-5 * time.Minute),
			NextRetryAt:          &fixture.nextRetryAt,
			RetryAttempts:        1,
			MaxAutoRetryAttempts: 8,
			RecoveryScope:        listingsubmission.RetryableRecoveryScopeTask,
			AutoResumeEnabled:    true,
		}, "queue full"); err != nil {
			t.Fatalf("MarkBlockedRetryable(%s) error = %v", fixture.id, err)
		}
	}

	submitted := make([]string, 0, 2)
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo: repo,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(taskID string) error {
				submitted = append(submitted, taskID)
				return nil
			})
		},
		now: func() time.Time { return now },
	})

	recovered, err := svc.RunRecoverySweep(ctx, now, 20)
	if err != nil {
		t.Fatalf("RunRecoverySweep() error = %v", err)
	}
	if recovered != 1 {
		t.Fatalf("RunRecoverySweep() recovered = %d, want 1", recovered)
	}
	if len(submitted) != 1 || submitted[0] != "task-due" {
		t.Fatalf("submitted = %v, want [task-due]", submitted)
	}

	dueTask, err := repo.GetTask(ctx, "task-due")
	if err != nil {
		t.Fatalf("GetTask(task-due) error = %v", err)
	}
	if dueTask.Status != TaskStatusPending {
		t.Fatalf("task-due status = %q, want %q", dueTask.Status, TaskStatusPending)
	}

	futureTask, err := repo.GetTask(ctx, "task-future")
	if err != nil {
		t.Fatalf("GetTask(task-future) error = %v", err)
	}
	if futureTask.Status != TaskStatusBlockedRetryable {
		t.Fatalf("task-future status = %q, want %q", futureTask.Status, TaskStatusBlockedRetryable)
	}
}

func TestBulkRecoverTasksSkipsNotRecoverableItems(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	ctx := WithTenantID(context.Background(), "tenant-bulk")
	now := time.Date(2026, 6, 6, 16, 0, 0, 0, time.UTC)
	dueRetryAt := now.Add(-2 * time.Minute)
	futureRetryAt := now.Add(2 * time.Minute)

	for _, fixture := range []struct {
		id          string
		nextRetryAt time.Time
	}{
		{id: "task-bulk-due", nextRetryAt: dueRetryAt},
		{id: "task-bulk-future", nextRetryAt: futureRetryAt},
	} {
		task := &Task{
			ID:        fixture.id,
			TenantID:  "tenant-bulk",
			Status:    TaskStatusPending,
			Request:   &GenerateRequest{TenantID: "tenant-bulk", Platforms: []string{"amazon"}, Text: fixture.id},
			CreatedAt: now.Add(-10 * time.Minute),
			UpdatedAt: now.Add(-10 * time.Minute),
		}
		if err := repo.CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask(%s) error = %v", fixture.id, err)
		}
		if err := repo.MarkBlockedRetryable(ctx, task.ID, &RetryableBlock{
			ReasonCode:           listingsubmission.RetryableReasonCodeWorkerQueueBackpressure,
			ReasonMessage:        "queue full",
			BlockedAt:            now.Add(-5 * time.Minute),
			NextRetryAt:          &fixture.nextRetryAt,
			RetryAttempts:        1,
			MaxAutoRetryAttempts: 8,
			RecoveryScope:        listingsubmission.RetryableRecoveryScopeTask,
			AutoResumeEnabled:    true,
		}, "queue full"); err != nil {
			t.Fatalf("MarkBlockedRetryable(%s) error = %v", fixture.id, err)
		}
	}

	submitted := make([]string, 0, 2)
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo: repo,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(taskID string) error {
				submitted = append(submitted, taskID)
				return nil
			})
		},
		now: func() time.Time { return now },
	})

	recovered, err := svc.BulkRecoverTasks(ctx, &RecoverBlockedTasksQuery{
		DueBefore: now,
		RecoverAt: now,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("BulkRecoverTasks() error = %v", err)
	}
	if recovered != 1 {
		t.Fatalf("BulkRecoverTasks() recovered = %d, want 1", recovered)
	}
	if len(submitted) != 1 || submitted[0] != "task-bulk-due" {
		t.Fatalf("submitted = %v, want [task-bulk-due]", submitted)
	}

	futureTask, err := repo.GetTask(ctx, "task-bulk-future")
	if err != nil {
		t.Fatalf("GetTask(task-bulk-future) error = %v", err)
	}
	if futureTask.Status != TaskStatusBlockedRetryable {
		t.Fatalf("task-bulk-future status = %q, want %q", futureTask.Status, TaskStatusBlockedRetryable)
	}
}

func TestRecoverTaskNowRejectsWhenSubmitterUnavailable(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	ctx := WithTenantID(context.Background(), "tenant-missing-submitter")
	now := time.Date(2026, 6, 6, 16, 30, 0, 0, time.UTC)
	nextRetryAt := now.Add(-time.Minute)
	createBlockedRecoveryTask(t, repo, ctx, "task-no-submitter", now, nextRetryAt, RetryableBlock{
		RetryAttempts:        1,
		MaxAutoRetryAttempts: 8,
		AutoResumeEnabled:    true,
	})

	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo: repo,
		now:  func() time.Time { return now },
	})

	if _, err := svc.RecoverTaskNow(ctx, "task-no-submitter"); !errors.Is(err, ErrTaskRecoveryUnavailable) {
		t.Fatalf("RecoverTaskNow() error = %v, want ErrTaskRecoveryUnavailable", err)
	}

	stored, err := repo.GetTask(ctx, "task-no-submitter")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.Status != TaskStatusBlockedRetryable {
		t.Fatalf("status = %q, want %q", stored.Status, TaskStatusBlockedRetryable)
	}
	if stored.RetryableBlock == nil || stored.RetryableBlock.LastRetryAt != nil {
		t.Fatalf("RetryableBlock = %+v, want blocked state unchanged", stored.RetryableBlock)
	}
}

func TestRecoverTaskNowRejectsNonRecoverableTask(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	ctx := WithTenantID(context.Background(), "tenant-invalid")
	now := time.Date(2026, 6, 6, 16, 45, 0, 0, time.UTC)
	task := &Task{
		ID:        "task-invalid",
		TenantID:  "tenant-invalid",
		Status:    TaskStatusPending,
		Request:   &GenerateRequest{TenantID: "tenant-invalid", Platforms: []string{"amazon"}, Text: "invalid"},
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-time.Minute),
	}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	submitted := false
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo: repo,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(taskID string) error {
				submitted = true
				return nil
			})
		},
		now: func() time.Time { return now },
	})

	if _, err := svc.RecoverTaskNow(ctx, task.ID); !errors.Is(err, ErrTaskNotRecoverable) {
		t.Fatalf("RecoverTaskNow() error = %v, want ErrTaskNotRecoverable", err)
	}
	if submitted {
		t.Fatal("submitter was called for non-recoverable task")
	}
}

func TestRecoverTaskNowReblocksRetryableSubmitFailures(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	ctx := WithTenantID(context.Background(), "tenant-reblock")
	now := time.Date(2026, 6, 6, 17, 0, 0, 0, time.UTC)
	nextRetryAt := now.Add(-time.Minute)
	task := &Task{
		ID:        "task-reblock",
		TenantID:  "tenant-reblock",
		Status:    TaskStatusPending,
		Request:   &GenerateRequest{TenantID: "tenant-reblock", Platforms: []string{"amazon"}, Text: "reblock"},
		CreatedAt: now.Add(-10 * time.Minute),
		UpdatedAt: now.Add(-10 * time.Minute),
	}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := repo.MarkBlockedRetryable(ctx, task.ID, &RetryableBlock{
		ReasonCode:           listingsubmission.RetryableReasonCodeWorkerQueueBackpressure,
		ReasonMessage:        "queue full",
		BlockedAt:            now.Add(-5 * time.Minute),
		NextRetryAt:          &nextRetryAt,
		RetryAttempts:        2,
		MaxAutoRetryAttempts: 8,
		RecoveryScope:        listingsubmission.RetryableRecoveryScopeTask,
		AutoResumeEnabled:    true,
	}, "queue full"); err != nil {
		t.Fatalf("MarkBlockedRetryable() error = %v", err)
	}

	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo: repo,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(string) error {
				return errors.New("queue full")
			})
		},
		now: func() time.Time { return now },
	})

	if _, err := svc.RecoverTaskNow(ctx, task.ID); err == nil {
		t.Fatal("RecoverTaskNow() error = nil, want submit failure")
	}

	stored, err := repo.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.Status != TaskStatusBlockedRetryable {
		t.Fatalf("status = %q, want %q", stored.Status, TaskStatusBlockedRetryable)
	}
	if stored.RetryableBlock == nil || stored.RetryableBlock.NextRetryAt == nil {
		t.Fatalf("RetryableBlock = %+v, want NextRetryAt after reblock", stored.RetryableBlock)
	}
	if stored.RetryableBlock.RetryAttempts != 3 {
		t.Fatalf("RetryAttempts = %d, want 3", stored.RetryableBlock.RetryAttempts)
	}
	if !stored.RetryableBlock.NextRetryAt.Equal(now.Add(listingsubmission.BoundedEnqueueRetryDelay(3))) {
		t.Fatalf("NextRetryAt = %v, want %v", stored.RetryableBlock.NextRetryAt, now.Add(listingsubmission.BoundedEnqueueRetryDelay(3)))
	}
}

func TestRecoverTaskNowReblocksRetryableSubmitFailuresAtMaxAutoRetryAttempts(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	ctx := WithTenantID(context.Background(), "tenant-max-attempts")
	now := time.Date(2026, 6, 6, 17, 30, 0, 0, time.UTC)
	nextRetryAt := now.Add(-time.Minute)
	createBlockedRecoveryTask(t, repo, ctx, "task-max-attempts", now, nextRetryAt, RetryableBlock{
		RetryAttempts:        2,
		MaxAutoRetryAttempts: 3,
		AutoResumeEnabled:    true,
	})

	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo: repo,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(string) error { return errors.New("queue full") })
		},
		now: func() time.Time { return now },
	})

	if _, err := svc.RecoverTaskNow(ctx, "task-max-attempts"); err == nil {
		t.Fatal("RecoverTaskNow() error = nil, want retryable submit failure")
	}

	stored, err := repo.GetTask(ctx, "task-max-attempts")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.RetryableBlock == nil {
		t.Fatal("RetryableBlock = nil, want persisted capped retry metadata")
	}
	if stored.RetryableBlock.RetryAttempts != 3 {
		t.Fatalf("RetryAttempts = %d, want 3", stored.RetryableBlock.RetryAttempts)
	}
	if !stored.RetryableBlock.AutoRetryPaused {
		t.Fatal("AutoRetryPaused = false, want true after max attempts reached")
	}
	if stored.RetryableBlock.NextRetryAt != nil {
		t.Fatalf("NextRetryAt = %v, want nil when auto retry is paused at cap", stored.RetryableBlock.NextRetryAt)
	}
}

func TestRecoverTaskNowRestoresBlockedStateWhenReblockPersistenceFails(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	ctx := WithTenantID(context.Background(), "tenant-restore")
	now := time.Date(2026, 6, 6, 17, 40, 0, 0, time.UTC)
	nextRetryAt := now.Add(-time.Minute)
	originalBlock := RetryableBlock{
		ReasonCode:           listingsubmission.RetryableReasonCodeWorkerQueueBackpressure,
		ReasonMessage:        "queue full",
		BlockedAt:            now.Add(-5 * time.Minute),
		RetryAttempts:        2,
		MaxAutoRetryAttempts: 8,
		RecoveryScope:        listingsubmission.RetryableRecoveryScopeTask,
		AutoResumeEnabled:    true,
	}
	createBlockedRecoveryTask(t, repo, ctx, "task-restore", now, nextRetryAt, originalBlock)
	repo.markBlockedRetryableCallCount = 0
	repo.markBlockedRetryableErrors = []error{errors.New("persist blocked state failed"), nil}

	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo: repo,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(string) error { return errors.New("queue full") })
		},
		now: func() time.Time { return now },
	})

	if _, err := svc.RecoverTaskNow(ctx, "task-restore"); err == nil {
		t.Fatal("RecoverTaskNow() error = nil, want submit failure with persistence error")
	}

	stored, err := repo.GetTask(ctx, "task-restore")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.Status != TaskStatusBlockedRetryable {
		t.Fatalf("status = %q, want %q after rollback restore", stored.Status, TaskStatusBlockedRetryable)
	}
	if stored.RetryableBlock == nil {
		t.Fatal("RetryableBlock = nil, want restored blocked metadata")
	}
	if stored.RetryableBlock.RetryAttempts != originalBlock.RetryAttempts {
		t.Fatalf("RetryAttempts = %d, want original %d after restore", stored.RetryableBlock.RetryAttempts, originalBlock.RetryAttempts)
	}
	if stored.RetryableBlock.NextRetryAt == nil || !stored.RetryableBlock.NextRetryAt.Equal(nextRetryAt) {
		t.Fatalf("NextRetryAt = %v, want original %v after restore", stored.RetryableBlock.NextRetryAt, nextRetryAt)
	}
	if repo.markBlockedRetryableCallCount != 2 {
		t.Fatalf("MarkBlockedRetryable calls = %d, want 2 for reblock then rollback restore", repo.markBlockedRetryableCallCount)
	}
}

func TestSubmissionReblockedRetryableBlockPreservesManualPauseAndAutoResumeDisabled(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 6, 17, 45, 0, 0, time.UTC)
	previous := &RetryableBlock{
		ReasonCode:           listingsubmission.RetryableReasonCodeWorkerQueueBackpressure,
		ReasonMessage:        "queue full",
		BlockedAt:            now.Add(-5 * time.Minute),
		RetryAttempts:        1,
		MaxAutoRetryAttempts: 8,
		RecoveryScope:        listingsubmission.RetryableRecoveryScopeTask,
		AutoResumeEnabled:    false,
		AutoRetryPaused:      true,
	}
	classified, ok := classifyRetryableTaskFailure(errors.New("queue full"))
	if !ok {
		t.Fatal("classifyRetryableTaskFailure() = not retryable, want queue full classification")
	}

	reblocked := adaptSubmissionRetryableBlock(listingsubmission.BuildReblockedRetryableBlock(
		adaptRetryableBlockState(previous),
		adaptRetryableBlockState(classified),
		now,
		listingsubmission.RetryableRecoveryScopeTask,
	))
	if reblocked.AutoResumeEnabled {
		t.Fatal("AutoResumeEnabled = true, want preserved false")
	}
	if !reblocked.AutoRetryPaused {
		t.Fatal("AutoRetryPaused = false, want preserved true")
	}
	if reblocked.NextRetryAt != nil {
		t.Fatalf("NextRetryAt = %v, want nil while pause is preserved", reblocked.NextRetryAt)
	}
}

type taskRecoveryServiceTestRepo struct {
	tasks                         map[string]*Task
	markBlockedRetryableErrors    []error
	markBlockedRetryableCallCount int
}

func newTaskRecoveryServiceTestRepo() *taskRecoveryServiceTestRepo {
	return &taskRecoveryServiceTestRepo{tasks: map[string]*Task{}}
}

func createBlockedRecoveryTask(t *testing.T, repo *taskRecoveryServiceTestRepo, ctx context.Context, taskID string, now time.Time, nextRetryAt time.Time, block RetryableBlock) {
	t.Helper()

	task := &Task{
		ID:        taskID,
		TenantID:  TenantIDFromContext(ctx),
		Status:    TaskStatusPending,
		Request:   &GenerateRequest{TenantID: TenantIDFromContext(ctx), Platforms: []string{"amazon"}, Text: taskID},
		CreatedAt: now.Add(-10 * time.Minute),
		UpdatedAt: now.Add(-10 * time.Minute),
	}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask(%s) error = %v", taskID, err)
	}
	block.ReasonCode = firstNonEmpty(block.ReasonCode, listingsubmission.RetryableReasonCodeWorkerQueueBackpressure)
	block.ReasonMessage = firstNonEmpty(block.ReasonMessage, "queue full")
	block.BlockedAt = firstNonZeroTime(block.BlockedAt, now.Add(-5*time.Minute))
	if block.NextRetryAt == nil {
		block.NextRetryAt = timestampTaskRecoveryServiceTest(nextRetryAt)
	}
	if block.RecoveryScope == "" {
		block.RecoveryScope = listingsubmission.RetryableRecoveryScopeTask
	}
	if err := repo.MarkBlockedRetryable(ctx, taskID, &block, block.ReasonMessage); err != nil {
		t.Fatalf("MarkBlockedRetryable(%s) error = %v", taskID, err)
	}
}

func (r *taskRecoveryServiceTestRepo) CreateTask(_ context.Context, task *Task) error {
	copied := *task
	copied.RetryableBlock = cloneRetryableBlock(task.RetryableBlock)
	r.tasks[task.ID] = &copied
	return nil
}

func (r *taskRecoveryServiceTestRepo) GetTask(_ context.Context, taskID string) (*Task, error) {
	task, ok := r.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}
	copied := *task
	copied.RetryableBlock = cloneRetryableBlock(task.RetryableBlock)
	return &copied, nil
}

func (r *taskRecoveryServiceTestRepo) ListTasks(context.Context, *TaskListQuery) ([]Task, int64, error) {
	return nil, 0, nil
}

func (r *taskRecoveryServiceTestRepo) MarkProcessing(context.Context, string) error {
	return nil
}

func (r *taskRecoveryServiceTestRepo) MarkCompleted(context.Context, string, *ListingKitResult) error {
	return nil
}

func (r *taskRecoveryServiceTestRepo) MarkNeedsReview(context.Context, string, *ListingKitResult, string) error {
	return nil
}

func (r *taskRecoveryServiceTestRepo) MarkFailed(_ context.Context, taskID string, errorMsg string) error {
	task, ok := r.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	task.Status = TaskStatusFailed
	task.Error = errorMsg
	task.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *taskRecoveryServiceTestRepo) MarkBlockedRetryable(_ context.Context, taskID string, block *RetryableBlock, errorMsg string) error {
	task, ok := r.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	r.markBlockedRetryableCallCount++
	if len(r.markBlockedRetryableErrors) > 0 {
		err := r.markBlockedRetryableErrors[0]
		r.markBlockedRetryableErrors = r.markBlockedRetryableErrors[1:]
		if err != nil {
			return err
		}
	}
	task.Status = TaskStatusBlockedRetryable
	task.Error = errorMsg
	task.RetryableBlock = cloneRetryableBlock(block)
	task.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *taskRecoveryServiceTestRepo) ListRecoverableTasks(_ context.Context, query *RecoverableTaskQuery) ([]Task, error) {
	dueBefore := time.Time{}
	if query != nil {
		dueBefore = query.DueBefore
	}
	items := make([]Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		if !taskRecoveryServiceTaskIsRecoverable(task, dueBefore, false) {
			continue
		}
		copied := *task
		copied.RetryableBlock = cloneRetryableBlock(task.RetryableBlock)
		items = append(items, copied)
	}
	if query != nil && query.Limit > 0 && len(items) > query.Limit {
		items = items[:query.Limit]
	}
	return items, nil
}

func (r *taskRecoveryServiceTestRepo) RecoverBlockedTaskNow(_ context.Context, taskID string, recoveredAt time.Time) error {
	task, ok := r.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	force := recoveredAt.IsZero()
	effectiveRecoveredAt := firstNonZeroTime(recoveredAt, time.Now().UTC())
	if !taskRecoveryServiceTaskIsRecoverable(task, effectiveRecoveredAt, force) {
		return ErrTaskNotRecoverable
	}
	block := cloneRetryableBlock(task.RetryableBlock)
	if block == nil {
		block = &RetryableBlock{}
	}
	block.LastRetryAt = timestampTaskRecoveryServiceTest(effectiveRecoveredAt)
	block.NextRetryAt = nil
	block.AutoRetryPaused = false
	task.Status = TaskStatusPending
	task.Error = ""
	task.RetryableBlock = block
	task.UpdatedAt = effectiveRecoveredAt
	return nil
}

func (r *taskRecoveryServiceTestRepo) BulkRecoverBlockedTasks(ctx context.Context, query *RecoverBlockedTasksQuery) (int64, error) {
	var recovered int64
	items, err := r.ListRecoverableTasks(ctx, &RecoverableTaskQuery{
		DueBefore: query.DueBefore,
		Limit:     query.Limit,
	})
	if err != nil {
		return 0, err
	}
	for i := range items {
		if err := r.RecoverBlockedTaskNow(ctx, items[i].ID, query.RecoverAt); err != nil {
			if errors.Is(err, ErrTaskNotRecoverable) {
				continue
			}
			return recovered, err
		}
		recovered++
	}
	return recovered, nil
}

func (r *taskRecoveryServiceTestRepo) PrepareRetry(context.Context, string) error {
	return nil
}

func (r *taskRecoveryServiceTestRepo) IncrementRetryCount(context.Context, string) error {
	return nil
}

func (r *taskRecoveryServiceTestRepo) SaveTaskResult(context.Context, string, *ListingKitResult) error {
	return nil
}

func taskRecoveryServiceTaskIsRecoverable(task *Task, dueBefore time.Time, force bool) bool {
	if task == nil || task.Status != TaskStatusBlockedRetryable {
		return false
	}
	if force {
		return true
	}
	block := task.RetryableBlock
	if block == nil || !block.AutoResumeEnabled || block.AutoRetryPaused || block.NextRetryAt == nil {
		return false
	}
	if dueBefore.IsZero() {
		return true
	}
	return !block.NextRetryAt.After(dueBefore)
}

func timestampTaskRecoveryServiceTest(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copied := value
	return &copied
}

func firstNonZeroTime(value time.Time, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value
}
