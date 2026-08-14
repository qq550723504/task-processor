package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/listingkit/core"
)

type recordingRecoveryUsageSettlement struct {
	commitCalls int
	commitErr   error
}

func (s *recordingRecoveryUsageSettlement) ReserveGeneration(context.Context, string, string, time.Time) (GenerationUsageReservation, error) {
	return GenerationUsageReservation{}, nil
}

func (s *recordingRecoveryUsageSettlement) CommitGeneration(context.Context, string, string) error {
	s.commitCalls++
	return s.commitErr
}

func (s *recordingRecoveryUsageSettlement) ReleaseGeneration(context.Context, string, string, string) error {
	return nil
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
	settlement := &recordingRecoveryUsageSettlement{commitErr: errors.New("ledger unavailable")}
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
