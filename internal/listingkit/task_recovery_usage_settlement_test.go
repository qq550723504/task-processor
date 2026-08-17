package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/listingkit/core"
)

type recordingRecoveryUsageSettlement struct {
	commitCalls   int
	commitErr     error
	commitTenant  string
	releaseCalls  int
	releaseErr    error
	releaseTenant string
	lookupState   GenerationUsageEventState
	lookupFound   bool
	lookupErr     error
}

func (s *recordingRecoveryUsageSettlement) ReserveGeneration(context.Context, string, string, time.Time) (GenerationUsageReservation, error) {
	return GenerationUsageReservation{}, nil
}

func (s *recordingRecoveryUsageSettlement) CommitGeneration(_ context.Context, tenantID, _ string) error {
	s.commitCalls++
	s.commitTenant = tenantID
	return s.commitErr
}

func (s *recordingRecoveryUsageSettlement) ReleaseGeneration(_ context.Context, tenantID, _, _ string) error {
	s.releaseCalls++
	s.releaseTenant = tenantID
	return s.releaseErr
}

func (s *recordingRecoveryUsageSettlement) LookupGeneration(context.Context, string, string) (GenerationUsageEventState, bool, error) {
	return s.lookupState, s.lookupFound, s.lookupErr
}

func (r *taskRecoveryServiceTestRepo) ResolveUsageSettlement(_ context.Context, taskID string) error {
	task, ok := r.tasks[taskID]
	if !ok {
		return core.ErrTaskNotFound
	}
	if len(r.resolveUsageSettlementErrors) > 0 {
		err := r.resolveUsageSettlementErrors[0]
		r.resolveUsageSettlementErrors = r.resolveUsageSettlementErrors[1:]
		if err != nil {
			return err
		}
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

func TestRecoverTaskNowReblocksCommitWhenFinalizationPersistenceFails(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	ctx := WithTenantID(context.Background(), "tenant-usage-finalize-commit")
	task := &Task{ID: "task-usage-finalize-commit", TenantID: "tenant-usage-finalize-commit", Status: core.TaskStatusCompleted, Result: &ListingKitResult{Status: string(core.TaskStatusCompleted)}, CreatedAt: time.Now().UTC()}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := repo.MarkBlockedRetryable(ctx, task.ID, &RetryableBlock{ReasonCode: usageCommitPendingReason, NextRetryAt: timestampTaskRecoveryServiceTest(time.Now().Add(-time.Minute)), AutoResumeEnabled: true}, "usage settlement pending"); err != nil {
		t.Fatalf("MarkBlockedRetryable() error = %v", err)
	}
	repo.resolveUsageSettlementErrors = []error{errors.New("task store unavailable")}
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{repo: repo, generationUsage: &recordingRecoveryUsageSettlement{}})
	if _, err := svc.RecoverTaskNow(ctx, task.ID); err == nil {
		t.Fatal("RecoverTaskNow() error = nil, want finalization persistence error")
	}
	got, err := repo.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got.RetryableBlock == nil || got.RetryableBlock.ReasonCode != usageCommitPendingReason || got.RetryableBlock.RetryAttempts != 1 || got.RetryableBlock.NextRetryAt == nil {
		t.Fatalf("task after finalization failure = %#v, want reblocked usage_commit_pending", got)
	}
}

func TestRecoverTaskNowReblocksReleaseWhenFinalizationPersistenceFails(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	ctx := WithTenantID(context.Background(), "tenant-usage-finalize-release")
	task := &Task{ID: "task-usage-finalize-release", TenantID: "tenant-usage-finalize-release", Status: core.TaskStatusProcessing, Error: "listing kit generation result persistence failed", CreatedAt: time.Now().UTC()}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := repo.MarkBlockedRetryable(ctx, task.ID, &RetryableBlock{ReasonCode: usageReleasePendingReason, NextRetryAt: timestampTaskRecoveryServiceTest(time.Now().Add(-time.Minute)), AutoResumeEnabled: true}, "usage release pending"); err != nil {
		t.Fatalf("MarkBlockedRetryable() error = %v", err)
	}
	repo.markFailedErrors = []error{errors.New("task store unavailable"), errors.New("task store unavailable"), errors.New("task store unavailable")}
	settlement := &recordingRecoveryUsageSettlement{}
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{repo: repo, generationUsage: settlement})
	if _, err := svc.RecoverTaskNow(ctx, task.ID); err == nil {
		t.Fatal("RecoverTaskNow() error = nil, want finalization persistence error")
	}
	got, err := repo.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got.RetryableBlock == nil || got.RetryableBlock.ReasonCode != usageReleasePendingReason || got.RetryableBlock.RetryAttempts != 1 || got.RetryableBlock.NextRetryAt == nil {
		t.Fatalf("task after finalization failure = %#v, want reblocked usage_release_pending", got)
	}
	if settlement.releaseCalls != 1 {
		t.Fatalf("release calls = %d, want 1", settlement.releaseCalls)
	}
}

func TestRecoverTaskNowHandlesTerminalPersistenceWithoutProviderSubmit(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	ctx := WithTenantID(context.Background(), "tenant-terminal-persistence")
	task := &Task{ID: "task-terminal-persistence", TenantID: "tenant-terminal-persistence", Status: core.TaskStatusProcessing, Error: "listing kit terminal state persistence is pending", CreatedAt: time.Now().UTC()}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := repo.MarkBlockedRetryable(ctx, task.ID, &RetryableBlock{ReasonCode: terminalPersistencePendingReason, NextRetryAt: timestampTaskRecoveryServiceTest(time.Now().Add(-time.Minute)), AutoResumeEnabled: true}, task.Error); err != nil {
		t.Fatalf("MarkBlockedRetryable() error = %v", err)
	}
	submitted := 0
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo: repo,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(string) error { submitted++; return nil })
		},
	})
	recovered, err := svc.RecoverTaskNow(ctx, task.ID)
	if err != nil {
		t.Fatalf("RecoverTaskNow() error = %v", err)
	}
	if recovered.Status != core.TaskStatusFailed || recovered.RetryableBlock != nil || submitted != 0 {
		t.Fatalf("recovered task = %#v, submitted = %d, want failed persistence-only recovery", recovered, submitted)
	}
}

func TestRecoverTaskNowRestoresCommittedReplayResultWithoutProviderSubmit(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	ctx := WithTenantID(context.Background(), "tenant-committed-replay")
	task := &Task{ID: "task-committed-replay", TenantID: "tenant-committed-replay", Status: core.TaskStatusProcessing, Result: &ListingKitResult{Status: string(core.TaskStatusCompleted)}, CreatedAt: time.Now().UTC()}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := repo.MarkBlockedRetryable(ctx, task.ID, &RetryableBlock{ReasonCode: committedReplayPersistencePendingReason, NextRetryAt: timestampTaskRecoveryServiceTest(time.Now().Add(-time.Minute)), AutoResumeEnabled: true}, "committed replay persistence pending"); err != nil {
		t.Fatalf("MarkBlockedRetryable() error = %v", err)
	}
	submitted := 0
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo: repo,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(string) error { submitted++; return nil })
		},
	})
	recovered, err := svc.RecoverTaskNow(ctx, task.ID)
	if err != nil {
		t.Fatalf("RecoverTaskNow() error = %v", err)
	}
	if recovered.Status != core.TaskStatusCompleted || recovered.RetryableBlock != nil || submitted != 0 {
		t.Fatalf("recovered task = %#v, submitted = %d, want completed persistence-only recovery", recovered, submitted)
	}
}

func TestRecoverTaskNowNormalizesBlankTenantForUsageSettlement(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	ctx := context.Background()
	task := &Task{ID: "task-default-tenant-settlement", TenantID: "", Status: core.TaskStatusCompleted, Result: &ListingKitResult{Status: string(core.TaskStatusCompleted)}, CreatedAt: time.Now().UTC()}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := repo.MarkBlockedRetryable(ctx, task.ID, &RetryableBlock{ReasonCode: usageCommitPendingReason}, "usage settlement pending"); err != nil {
		t.Fatalf("MarkBlockedRetryable() error = %v", err)
	}
	settlement := &recordingRecoveryUsageSettlement{}
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{repo: repo, generationUsage: settlement})
	if _, err := svc.RecoverTaskNow(ctx, task.ID); err != nil {
		t.Fatalf("RecoverTaskNow() error = %v", err)
	}
	if settlement.commitTenant != DefaultTenantID {
		t.Fatalf("commit tenant = %q, want default %q", settlement.commitTenant, DefaultTenantID)
	}
}

func TestRecoverTaskNowReblocksWithCleanupContextWhenSweepContextCanceled(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	repo.requireLiveBlockContext = true
	ctx, cancel := context.WithCancel(WithTenantID(context.Background(), "tenant-canceled-reblock"))
	cancel()
	now := time.Now().UTC()
	task := &Task{ID: "task-canceled-reblock", TenantID: "tenant-canceled-reblock", Status: core.TaskStatusCompleted, Result: &ListingKitResult{Status: string(core.TaskStatusCompleted)}, CreatedAt: now.Add(-time.Hour)}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	due := now.Add(-time.Minute)
	if err := repo.MarkBlockedRetryable(context.Background(), task.ID, &RetryableBlock{ReasonCode: usageCommitPendingReason, NextRetryAt: &due, AutoResumeEnabled: true}, "usage settlement pending"); err != nil {
		t.Fatalf("MarkBlockedRetryable() error = %v", err)
	}
	settlement := &recordingRecoveryUsageSettlement{commitErr: context.DeadlineExceeded}
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{repo: repo, generationUsage: settlement})
	if _, err := svc.RecoverTaskNow(ctx, task.ID); err == nil {
		t.Fatal("RecoverTaskNow() error = nil, want settlement error")
	}
	got, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got.RetryableBlock == nil || got.RetryableBlock.ReasonCode != usageCommitPendingReason || got.RetryableBlock.RetryAttempts != 1 || got.RetryableBlock.NextRetryAt == nil || !got.RetryableBlock.NextRetryAt.After(now) {
		t.Fatalf("task after canceled recovery = %#v, want reblocked settlement task", got)
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

func TestRunRecoverySweepFiltersFailedSettlementPagesBeforeProviderBatch(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	now := time.Now().UTC()
	ctx := WithTenantID(context.Background(), "tenant-usage-sweep-pages")
	settlementDue := now.Add(-2 * time.Minute)
	regularDue := now.Add(-time.Minute)
	for _, id := range []string{"task-usage-sweep-page-settle-1", "task-usage-sweep-page-settle-2"} {
		task := &Task{ID: id, TenantID: "tenant-usage-sweep-pages", Status: core.TaskStatusCompleted, Result: &ListingKitResult{Status: string(core.TaskStatusCompleted)}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}
		if err := repo.CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask(%s) error = %v", id, err)
		}
		if err := repo.MarkBlockedRetryable(ctx, id, &RetryableBlock{ReasonCode: usageCommitPendingReason, NextRetryAt: &settlementDue, AutoResumeEnabled: true}, "usage settlement pending"); err != nil {
			t.Fatalf("MarkBlockedRetryable(%s) error = %v", id, err)
		}
	}
	regular := &Task{ID: "task-usage-sweep-page-regular", TenantID: "tenant-usage-sweep-pages", Status: core.TaskStatusPending, Request: &GenerateRequest{TenantID: "tenant-usage-sweep-pages", Platforms: []string{"amazon"}}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}
	if err := repo.CreateTask(ctx, regular); err != nil {
		t.Fatalf("CreateTask(regular) error = %v", err)
	}
	if err := repo.MarkBlockedRetryable(ctx, regular.ID, &RetryableBlock{ReasonCode: "queue_backpressure", NextRetryAt: &regularDue, AutoResumeEnabled: true}, "queue full"); err != nil {
		t.Fatalf("MarkBlockedRetryable(regular) error = %v", err)
	}
	submitted := make([]string, 0, 1)
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo:            repo,
		generationUsage: &recordingRecoveryUsageSettlement{commitErr: errors.New("ledger unavailable")},
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(taskID string) error { submitted = append(submitted, taskID); return nil })
		},
	})
	recovered, err := svc.RunRecoverySweep(ctx, now, 1)
	if recovered != 1 || err == nil {
		t.Fatalf("RunRecoverySweep() = (%d, %v), want regular recovery plus settlement error", recovered, err)
	}
	if len(submitted) != 1 || submitted[0] != regular.ID {
		t.Fatalf("submitted = %v, want [%s]", submitted, regular.ID)
	}
}

func TestRunRecoverySweepHonorsLimitAfterUsageSettlement(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	now := time.Now().UTC()
	ctx := WithTenantID(context.Background(), "tenant-usage-sweep-limit")
	settlementDue := now.Add(-2 * time.Minute)
	regularDue := now.Add(-time.Minute)
	for _, task := range []*Task{
		{ID: "task-usage-sweep-limit-settle", TenantID: "tenant-usage-sweep-limit", Status: core.TaskStatusCompleted, Result: &ListingKitResult{Status: string(core.TaskStatusCompleted)}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
		{ID: "task-usage-sweep-limit-regular", TenantID: "tenant-usage-sweep-limit", Status: core.TaskStatusPending, Request: &GenerateRequest{TenantID: "tenant-usage-sweep-limit", Platforms: []string{"amazon"}}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
	} {
		if err := repo.CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask(%s) error = %v", task.ID, err)
		}
	}
	if err := repo.MarkBlockedRetryable(ctx, "task-usage-sweep-limit-settle", &RetryableBlock{ReasonCode: "usage_commit_pending", NextRetryAt: &settlementDue, AutoResumeEnabled: true}, "usage settlement pending"); err != nil {
		t.Fatalf("MarkBlockedRetryable(settlement) error = %v", err)
	}
	if err := repo.MarkBlockedRetryable(ctx, "task-usage-sweep-limit-regular", &RetryableBlock{ReasonCode: "queue_backpressure", NextRetryAt: &regularDue, AutoResumeEnabled: true}, "queue full"); err != nil {
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

func TestRunRecoverySweepClaimsExpiredGenerationReservationForReconciliation(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	now := time.Date(2026, 8, 17, 5, 0, 0, 0, time.UTC)
	ctx := WithTenantID(context.Background(), "tenant-expired-generation")
	leaseUntil := now.Add(-time.Minute)
	task := &Task{
		ID:                                   "task-expired-generation",
		TenantID:                             "tenant-expired-generation",
		BillingTenantID:                      "billing-expired-generation",
		Status:                               core.TaskStatusProcessing,
		GenerationUsageReservationState:      GenerationUsageReservationStateReserved,
		GenerationUsageReservationLeaseUntil: &leaseUntil,
		CreatedAt:                            now.Add(-time.Hour),
		UpdatedAt:                            now.Add(-time.Hour),
	}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	settlement := &recordingRecoveryUsageSettlement{lookupState: GenerationUsageEventReserved, lookupFound: true}
	submitted := 0
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo:            repo,
		generationUsage: settlement,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(string) error { submitted++; return nil })
		},
		now: func() time.Time { return now },
	})

	recovered, err := svc.RunRecoverySweep(ctx, now, 10)
	if err != nil {
		t.Fatalf("RunRecoverySweep() error = %v", err)
	}
	if recovered != 1 || settlement.releaseCalls != 0 || submitted != 0 {
		t.Fatalf("recovered/release/submitted = (%d, %d, %d), want (1, 0, 0)", recovered, settlement.releaseCalls, submitted)
	}
	got, err := repo.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got.Status != core.TaskStatusBlockedRetryable || got.RetryableBlock == nil || got.RetryableBlock.ReasonCode != generationUsageReconciliationPendingReason || got.RetryableBlock.AutoResumeEnabled {
		t.Fatalf("recovered task = %#v, want reconciliation-only block", got)
	}
	if got.GenerationUsageReservationState == "" || got.GenerationUsageReservationLeaseUntil == nil {
		t.Fatalf("recovered reservation = (%q, %v), want retained for reconciliation", got.GenerationUsageReservationState, got.GenerationUsageReservationLeaseUntil)
	}
}

func TestRunRecoverySweepCommitsTerminalTaskWithExpiredGenerationReservation(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	now := time.Date(2026, 8, 17, 5, 15, 0, 0, time.UTC)
	ctx := WithTenantID(context.Background(), "tenant-terminal-generation")
	leaseUntil := now.Add(-time.Minute)
	task := &Task{ID: "task-terminal-generation", TenantID: "tenant-terminal-generation", BillingTenantID: "billing-terminal-generation", Status: core.TaskStatusCompleted, Result: &ListingKitResult{Status: string(core.TaskStatusCompleted)}, GenerationUsageReservationState: GenerationUsageReservationStateReserved, GenerationUsageReservationLeaseUntil: &leaseUntil, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	settlement := &recordingRecoveryUsageSettlement{}
	submitted := 0
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo:            repo,
		generationUsage: settlement,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(string) error { submitted++; return nil })
		},
		now: func() time.Time { return now },
	})

	recovered, err := svc.RunRecoverySweep(ctx, now, 10)
	if err != nil {
		t.Fatalf("RunRecoverySweep() error = %v", err)
	}
	if recovered != 1 || settlement.commitCalls != 1 || submitted != 0 {
		t.Fatalf("recovered/commit/submitted = (%d, %d, %d), want (1, 1, 0)", recovered, settlement.commitCalls, submitted)
	}
	got, err := repo.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got.Status != core.TaskStatusCompleted || got.GenerationUsageReservationState != "" || got.GenerationUsageReservationLeaseUntil != nil {
		t.Fatalf("terminal task after settlement = %#v, want completed with cleared reservation", got)
	}
}

func TestRecoverTaskNowRejectsGenerationUsageReconciliationWithoutProviderSubmit(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	ctx := WithTenantID(context.Background(), "tenant-reconciliation-manual")
	now := time.Date(2026, 8, 17, 6, 0, 0, 0, time.UTC)
	task := &Task{ID: "task-reconciliation-manual", TenantID: "tenant-reconciliation-manual", Status: core.TaskStatusProcessing, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := repo.MarkBlockedRetryable(ctx, task.ID, generationUsageReconciliationBlock(now), "requires reconciliation"); err != nil {
		t.Fatalf("MarkBlockedRetryable() error = %v", err)
	}
	submitted := 0
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo: repo,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(string) error { submitted++; return nil })
		},
	})

	if _, err := svc.RecoverTaskNow(ctx, task.ID); !errors.Is(err, core.ErrTaskNotRecoverable) {
		t.Fatalf("RecoverTaskNow() error = %v, want ErrTaskNotRecoverable", err)
	}
	if submitted != 0 {
		t.Fatalf("provider submissions = %d, want 0", submitted)
	}
}

func TestRunRecoverySweepSkipsReservationRenewedAfterExpiryScan(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	now := time.Date(2026, 8, 17, 5, 30, 0, 0, time.UTC)
	ctx := WithTenantID(context.Background(), "tenant-renewed-generation")
	leaseUntil := now.Add(-time.Minute)
	task := &Task{ID: "task-renewed-generation", TenantID: "tenant-renewed-generation", Status: core.TaskStatusProcessing, GenerationUsageReservationState: GenerationUsageReservationStateReserved, GenerationUsageReservationLeaseUntil: &leaseUntil, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	repo.afterListExpired = func() {
		renewed := now.Add(time.Minute)
		repo.tasks[task.ID].GenerationUsageReservationLeaseUntil = &renewed
	}
	settlement := &recordingRecoveryUsageSettlement{}
	submitted := 0
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo:            repo,
		generationUsage: settlement,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(string) error { submitted++; return nil })
		},
		now: func() time.Time { return now },
	})

	recovered, err := svc.RunRecoverySweep(ctx, now, 10)
	if err != nil {
		t.Fatalf("RunRecoverySweep() error = %v", err)
	}
	if recovered != 0 || settlement.releaseCalls != 0 || submitted != 0 {
		t.Fatalf("recovered/release/submitted = (%d, %d, %d), want (0, 0, 0)", recovered, settlement.releaseCalls, submitted)
	}
	got, err := repo.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got.Status != core.TaskStatusProcessing || got.GenerationUsageReservationLeaseUntil == nil || !got.GenerationUsageReservationLeaseUntil.After(now) {
		t.Fatalf("task after renewed lease = %#v, want live processing reservation", got)
	}
}

func TestRunRecoverySweepBlocksExpiredGenerationReservationWhenLedgerLookupIsUncertain(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	now := time.Date(2026, 8, 17, 5, 0, 0, 0, time.UTC)
	ctx := WithTenantID(context.Background(), "tenant-uncertain-generation")
	leaseUntil := now.Add(-time.Minute)
	task := &Task{ID: "task-uncertain-generation", TenantID: "tenant-uncertain-generation", BillingTenantID: "billing-uncertain-generation", Status: core.TaskStatusProcessing, GenerationUsageReservationState: GenerationUsageReservationStateReserved, GenerationUsageReservationLeaseUntil: &leaseUntil, CreatedAt: now.Add(-time.Hour)}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	settlement := &recordingRecoveryUsageSettlement{lookupErr: errors.New("ledger unavailable")}
	submitted := 0
	svc := newTaskRecoveryService(taskRecoveryServiceConfig{
		repo:            repo,
		generationUsage: settlement,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(string) error { submitted++; return nil })
		},
		now: func() time.Time { return now },
	})

	recovered, err := svc.RunRecoverySweep(ctx, now, 10)
	if err != nil {
		t.Fatalf("RunRecoverySweep() error = %v", err)
	}
	if recovered != 1 || settlement.releaseCalls != 0 || submitted != 0 {
		t.Fatalf("recovered/release/submitted = (%d, %d, %d), want (1, 0, 0)", recovered, settlement.releaseCalls, submitted)
	}
	got, err := repo.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got.Status != core.TaskStatusBlockedRetryable || got.RetryableBlock == nil || got.RetryableBlock.ReasonCode != generationUsageReconciliationPendingReason {
		t.Fatalf("recovered task = %#v, want reconciliation block", got)
	}
	if got.GenerationUsageReservationState == "" || got.GenerationUsageReservationLeaseUntil == nil {
		t.Fatalf("recovered reservation = (%q, %v), want retained intent for reconciliation", got.GenerationUsageReservationState, got.GenerationUsageReservationLeaseUntil)
	}
}
