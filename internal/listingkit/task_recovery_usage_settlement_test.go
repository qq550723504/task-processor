package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/listingkit/core"
)

type recordingRecoveryUsageSettlement struct {
	commitCalls  int
	commitErr    error
	releaseCalls int
	releaseErr   error
}

func (s *recordingRecoveryUsageSettlement) ReserveGeneration(context.Context, string, string, time.Time) (GenerationUsageReservation, error) {
	return GenerationUsageReservation{}, nil
}

func (s *recordingRecoveryUsageSettlement) CommitGeneration(context.Context, string, string) error {
	s.commitCalls++
	return s.commitErr
}

func (s *recordingRecoveryUsageSettlement) ReleaseGeneration(context.Context, string, string, string) error {
	s.releaseCalls++
	return s.releaseErr
}

func (r *taskRecoveryServiceTestRepo) ResolveUsageSettlement(_ context.Context, taskID string) error {
	task, ok := r.tasks[taskID]
	if !ok {
		return core.ErrTaskNotFound
	}
	if task.RetryableBlock == nil || task.RetryableBlock.ReasonCode != "usage_commit_pending" || task.Result == nil {
		return core.ErrTaskNotRecoverable
	}
	task.Status = core.TaskStatus(task.Result.Status)
	task.RetryableBlock = nil
	task.Error = ""
	return nil
}

func TestRecoverTaskNowSettlesUsageCommitWithoutSubmittingTask(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	ctx := WithTenantID(context.Background(), "tenant-usage-recovery")
	task := &Task{ID: "task-usage-recovery", TenantID: "tenant-usage-recovery", Status: core.TaskStatusCompleted, Result: &ListingKitResult{Status: string(core.TaskStatusCompleted)}, CreatedAt: time.Now().UTC()}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := repo.MarkBlockedRetryable(ctx, task.ID, &RetryableBlock{ReasonCode: "usage_commit_pending"}, "usage settlement pending"); err != nil {
		t.Fatalf("MarkBlockedRetryable() error = %v", err)
	}
	settlement := &recordingRecoveryUsageSettlement{}
	submitted := 0
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo:            repo,
		generationUsage: settlement,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(string) error { submitted++; return nil })
		},
	})

	recovered, err := svc.RecoverTaskNow(ctx, task.ID)
	if err != nil {
		t.Fatalf("RecoverTaskNow() error = %v", err)
	}
	if recovered.Status != core.TaskStatusCompleted || recovered.RetryableBlock != nil {
		t.Fatalf("recovered task = %#v, want completed task with cleared block", recovered)
	}
	if settlement.commitCalls != 1 {
		t.Fatalf("commit calls = %d, want 1", settlement.commitCalls)
	}
	if submitted != 0 {
		t.Fatalf("submit calls = %d, want 0 for settlement-only recovery", submitted)
	}
}

func TestRecoverTaskNowLeavesUsageSettlementBlockedWhenCommitFails(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	ctx := WithTenantID(context.Background(), "tenant-usage-recovery-fail")
	task := &Task{ID: "task-usage-recovery-fail", TenantID: "tenant-usage-recovery-fail", Status: core.TaskStatusCompleted, Result: &ListingKitResult{Status: string(core.TaskStatusCompleted)}, CreatedAt: time.Now().UTC()}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := repo.MarkBlockedRetryable(ctx, task.ID, &RetryableBlock{ReasonCode: "usage_commit_pending"}, "usage settlement pending"); err != nil {
		t.Fatalf("MarkBlockedRetryable() error = %v", err)
	}
	settlement := &recordingRecoveryUsageSettlement{commitErr: errors.New("context deadline exceeded")}
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{repo: repo, generationUsage: settlement})

	if _, err := svc.RecoverTaskNow(ctx, task.ID); err == nil {
		t.Fatal("RecoverTaskNow() error = nil, want settlement failure")
	}
	got, err := repo.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got.Status != core.TaskStatusBlockedRetryable || got.RetryableBlock == nil || got.RetryableBlock.ReasonCode != "usage_commit_pending" {
		t.Fatalf("task after failed settlement = %#v, want unchanged block", got)
	}
}

func TestRunRecoverySweepSettlesUsageCommitWithoutSubmittingTask(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	now := time.Now().UTC()
	ctx := WithTenantID(context.Background(), "tenant-usage-sweep")
	task := &Task{ID: "task-usage-sweep", TenantID: "tenant-usage-sweep", Status: core.TaskStatusCompleted, Result: &ListingKitResult{Status: string(core.TaskStatusCompleted)}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	due := now.Add(-time.Minute)
	if err := repo.MarkBlockedRetryable(ctx, task.ID, &RetryableBlock{ReasonCode: "usage_commit_pending", NextRetryAt: &due, AutoResumeEnabled: true}, "usage settlement pending"); err != nil {
		t.Fatalf("MarkBlockedRetryable() error = %v", err)
	}
	settlement := &recordingRecoveryUsageSettlement{}
	submitted := 0
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo:            repo,
		generationUsage: settlement,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(string) error { submitted++; return nil })
		},
	})

	recovered, err := svc.RunRecoverySweep(ctx, now, 10)
	if err != nil {
		t.Fatalf("RunRecoverySweep() error = %v", err)
	}
	if recovered != 1 || settlement.commitCalls != 1 || submitted != 0 {
		t.Fatalf("recovered/commit/submitted = (%d, %d, %d), want (1, 1, 0)", recovered, settlement.commitCalls, submitted)
	}
}

func TestRunRecoverySweepBacksOffFailedUsageCommitSettlement(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	now := time.Now().UTC()
	ctx := WithTenantID(context.Background(), "tenant-usage-sweep-fail")
	task := &Task{ID: "task-usage-sweep-fail", TenantID: "tenant-usage-sweep-fail", Status: core.TaskStatusCompleted, Result: &ListingKitResult{Status: string(core.TaskStatusCompleted)}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	due := now.Add(-time.Minute)
	if err := repo.MarkBlockedRetryable(ctx, task.ID, &RetryableBlock{
		ReasonCode:           "usage_commit_pending",
		ReasonMessage:        "usage settlement is pending",
		BlockedAt:            now.Add(-10 * time.Minute),
		NextRetryAt:          &due,
		RetryAttempts:        1,
		MaxAutoRetryAttempts: 8,
		AutoResumeEnabled:    true,
	}, "usage settlement pending"); err != nil {
		t.Fatalf("MarkBlockedRetryable() error = %v", err)
	}
	settlement := &recordingRecoveryUsageSettlement{commitErr: errors.New("ledger unavailable")}
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{repo: repo, generationUsage: settlement})

	if recovered, err := svc.RunRecoverySweep(ctx, now, 10); recovered != 0 || err == nil {
		t.Fatalf("RunRecoverySweep() = (%d, %v), want no recovery and settlement error", recovered, err)
	}
	got, err := repo.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got.RetryableBlock == nil || got.RetryableBlock.ReasonCode != "usage_commit_pending" || got.RetryableBlock.RetryAttempts != 2 || got.RetryableBlock.NextRetryAt == nil || !got.RetryableBlock.NextRetryAt.After(now) {
		t.Fatalf("updated retryable block = %+v, want incremented attempt with future retry", got.RetryableBlock)
	}
}

func TestRunRecoverySweepContinuesUnrelatedRecoveryAfterSettlementFailure(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	now := time.Now().UTC()
	ctx := WithTenantID(context.Background(), "tenant-usage-sweep-continue")
	due := now.Add(-time.Minute)
	settlementTask := &Task{ID: "task-usage-sweep-continue-fail", TenantID: "tenant-usage-sweep-continue", Status: core.TaskStatusCompleted, Result: &ListingKitResult{Status: string(core.TaskStatusCompleted)}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}
	if err := repo.CreateTask(ctx, settlementTask); err != nil {
		t.Fatalf("CreateTask(settlement) error = %v", err)
	}
	if err := repo.MarkBlockedRetryable(ctx, settlementTask.ID, &RetryableBlock{ReasonCode: "usage_commit_pending", NextRetryAt: &due, AutoResumeEnabled: true}, "usage settlement pending"); err != nil {
		t.Fatalf("MarkBlockedRetryable(settlement) error = %v", err)
	}
	regularTask := &Task{ID: "task-usage-sweep-regular", TenantID: "tenant-usage-sweep-continue", Status: core.TaskStatusPending, Request: &GenerateRequest{TenantID: "tenant-usage-sweep-continue", Platforms: []string{"amazon"}}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}
	if err := repo.CreateTask(ctx, regularTask); err != nil {
		t.Fatalf("CreateTask(regular) error = %v", err)
	}
	if err := repo.MarkBlockedRetryable(ctx, regularTask.ID, &RetryableBlock{ReasonCode: "queue_backpressure", NextRetryAt: &due, AutoResumeEnabled: true}, "queue full"); err != nil {
		t.Fatalf("MarkBlockedRetryable(regular) error = %v", err)
	}

	settlement := &recordingRecoveryUsageSettlement{commitErr: errors.New("ledger unavailable")}
	submitted := make([]string, 0, 1)
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo:            repo,
		generationUsage: settlement,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(taskID string) error {
				submitted = append(submitted, taskID)
				return nil
			})
		},
	})

	recovered, err := svc.RunRecoverySweep(ctx, now, 10)
	if recovered != 1 || err == nil {
		t.Fatalf("RunRecoverySweep() = (%d, %v), want unrelated recovery plus settlement error", recovered, err)
	}
	if len(submitted) != 1 || submitted[0] != regularTask.ID {
		t.Fatalf("submitted = %v, want [%s]", submitted, regularTask.ID)
	}
}

func TestRunRecoverySweepHonorsLimitAfterUsageSettlement(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	now := time.Now().UTC()
	ctx := WithTenantID(context.Background(), "tenant-usage-sweep-limit")
	due := now.Add(-time.Minute)
	for _, task := range []*Task{
		{ID: "task-usage-sweep-limit-settle", TenantID: "tenant-usage-sweep-limit", Status: core.TaskStatusCompleted, Result: &ListingKitResult{Status: string(core.TaskStatusCompleted)}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
		{ID: "task-usage-sweep-limit-regular", TenantID: "tenant-usage-sweep-limit", Status: core.TaskStatusPending, Request: &GenerateRequest{TenantID: "tenant-usage-sweep-limit", Platforms: []string{"amazon"}}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
	} {
		if err := repo.CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask(%s) error = %v", task.ID, err)
		}
	}
	if err := repo.MarkBlockedRetryable(ctx, "task-usage-sweep-limit-settle", &RetryableBlock{ReasonCode: "usage_commit_pending", NextRetryAt: &due, AutoResumeEnabled: true}, "usage settlement pending"); err != nil {
		t.Fatalf("MarkBlockedRetryable(settlement) error = %v", err)
	}
	if err := repo.MarkBlockedRetryable(ctx, "task-usage-sweep-limit-regular", &RetryableBlock{ReasonCode: "queue_backpressure", NextRetryAt: &due, AutoResumeEnabled: true}, "queue full"); err != nil {
		t.Fatalf("MarkBlockedRetryable(regular) error = %v", err)
	}

	settlement := &recordingRecoveryUsageSettlement{}
	submitted := make([]string, 0, 1)
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo:            repo,
		generationUsage: settlement,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(taskID string) error {
				submitted = append(submitted, taskID)
				return nil
			})
		},
	})

	recovered, err := svc.RunRecoverySweep(ctx, now, 1)
	if err != nil {
		t.Fatalf("RunRecoverySweep() error = %v", err)
	}
	if recovered != 1 || settlement.commitCalls != 1 || len(submitted) != 0 {
		t.Fatalf("recovered/commit/submitted = (%d, %d, %d), want (1, 1, 0) with limit 1", recovered, settlement.commitCalls, len(submitted))
	}
}

func TestRunRecoverySweepReleasesUsageWithoutSubmittingTask(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	now := time.Now().UTC()
	ctx := WithTenantID(context.Background(), "tenant-usage-release-sweep")
	task := &Task{ID: "task-usage-release-sweep", TenantID: "tenant-usage-release-sweep", Status: core.TaskStatusProcessing, Error: "listing kit generation result persistence failed", CreatedAt: now.Add(-time.Hour), UpdatedAt: now}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	due := now.Add(-time.Minute)
	if err := repo.MarkBlockedRetryable(ctx, task.ID, &RetryableBlock{
		ReasonCode:           "usage_release_pending",
		ReasonMessage:        "usage release is pending",
		BlockedAt:            now.Add(-10 * time.Minute),
		NextRetryAt:          &due,
		RetryAttempts:        1,
		MaxAutoRetryAttempts: 8,
		AutoResumeEnabled:    true,
	}, "usage release is pending"); err != nil {
		t.Fatalf("MarkBlockedRetryable() error = %v", err)
	}
	settlement := &recordingRecoveryUsageSettlement{}
	submitted := 0
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo:            repo,
		generationUsage: settlement,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(string) error { submitted++; return nil })
		},
	})

	recovered, err := svc.RunRecoverySweep(ctx, now, 10)
	if err != nil {
		t.Fatalf("RunRecoverySweep() error = %v", err)
	}
	if recovered != 1 || settlement.releaseCalls != 1 || submitted != 0 {
		t.Fatalf("recovered/release/submitted = (%d, %d, %d), want (1, 1, 0)", recovered, settlement.releaseCalls, submitted)
	}
	got, err := repo.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got.Status != core.TaskStatusFailed || got.RetryableBlock != nil {
		t.Fatalf("released task = %#v, want failed task without settlement block", got)
	}
}
