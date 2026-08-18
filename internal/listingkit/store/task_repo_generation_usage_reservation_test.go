package store_test

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
)

func TestGenerationUsageReservationLeaseLifecycle(t *testing.T) {
	t.Parallel()

	for _, factory := range generationUsageReservationRepoFactories(t) {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			repo := factory.new(t)
			ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
			task := retryableTaskFixture("generation-usage-lease-lifecycle", time.Date(2026, 8, 17, 8, 0, 0, 0, time.UTC))
			if err := repo.CreateTask(ctx, task); err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			if err := repo.MarkProcessing(ctx, task.ID); err != nil {
				t.Fatalf("MarkProcessing() error = %v", err)
			}

			firstLease := time.Date(2026, 8, 17, 8, 10, 0, 0, time.UTC)
			if err := repo.BeginGenerationUsageReservation(ctx, task.ID, firstLease); err != nil {
				t.Fatalf("BeginGenerationUsageReservation() error = %v", err)
			}
			if err := repo.MarkGenerationUsageReserved(ctx, task.ID, firstLease); err != nil {
				t.Fatalf("MarkGenerationUsageReserved() error = %v", err)
			}

			renewedLease := firstLease.Add(10 * time.Minute)
			if err := repo.RenewGenerationUsageReservation(ctx, task.ID, renewedLease); err != nil {
				t.Fatalf("RenewGenerationUsageReservation() error = %v", err)
			}
			got, err := repo.GetTask(ctx, task.ID)
			if err != nil {
				t.Fatalf("GetTask() error = %v", err)
			}
			if got.GenerationUsageReservationState != listingkit.GenerationUsageReservationStateReserved {
				t.Fatalf("GenerationUsageReservationState = %q, want reserved", got.GenerationUsageReservationState)
			}
			if got.GenerationUsageReservationLeaseUntil == nil || !got.GenerationUsageReservationLeaseUntil.Equal(renewedLease) {
				t.Fatalf("GenerationUsageReservationLeaseUntil = %v, want %v", got.GenerationUsageReservationLeaseUntil, renewedLease)
			}

			if err := repo.ClearGenerationUsageReservation(ctx, task.ID); err != nil {
				t.Fatalf("ClearGenerationUsageReservation() error = %v", err)
			}
			got, err = repo.GetTask(ctx, task.ID)
			if err != nil {
				t.Fatalf("GetTask() after clear error = %v", err)
			}
			if got.GenerationUsageReservationState != "" || got.GenerationUsageReservationLeaseUntil != nil {
				t.Fatalf("reservation after clear = (%q, %v), want empty", got.GenerationUsageReservationState, got.GenerationUsageReservationLeaseUntil)
			}
		})
	}
}

func TestListExpiredGenerationUsageReservationsIncludesTerminalLeases(t *testing.T) {
	t.Parallel()

	for _, factory := range generationUsageReservationRepoFactories(t) {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			repo := factory.new(t)
			ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
			now := time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC)
			expired := createProcessingGenerationUsageReservation(t, repo, ctx, "generation-usage-expired", now.Add(-time.Minute))
			terminal := createProcessingGenerationUsageReservation(t, repo, ctx, "generation-usage-terminal", now.Add(-time.Minute))
			if err := repo.MarkCompleted(ctx, terminal.ID, &listingkit.ListingKitResult{Status: string(core.TaskStatusCompleted)}); err != nil {
				t.Fatalf("MarkCompleted() error = %v", err)
			}
			createProcessingGenerationUsageReservation(t, repo, ctx, "generation-usage-future", now.Add(time.Minute))
			pending := retryableTaskFixture("generation-usage-pending", now.Add(-time.Hour))
			pendingLease := now.Add(-time.Minute)
			pending.GenerationUsageReservationState = listingkit.GenerationUsageReservationStateReserved
			pending.GenerationUsageReservationLeaseUntil = &pendingLease
			if err := repo.CreateTask(ctx, pending); err != nil {
				t.Fatalf("CreateTask(pending) error = %v", err)
			}
			blocked := createProcessingGenerationUsageReservation(t, repo, ctx, "generation-usage-blocked", now.Add(-time.Minute))
			if err := repo.MarkBlockedRetryable(ctx, blocked.ID, &listingkit.RetryableBlock{ReasonCode: "test", AutoResumeEnabled: true}, "blocked"); err != nil {
				t.Fatalf("MarkBlockedRetryable() error = %v", err)
			}

			items, err := repo.ListExpiredGenerationUsageReservations(ctx, now, 10)
			if err != nil {
				t.Fatalf("ListExpiredGenerationUsageReservations() error = %v", err)
			}
			gotIDs := make([]string, 0, len(items))
			for _, item := range items {
				gotIDs = append(gotIDs, item.ID)
			}
			sort.Strings(gotIDs)
			if want := []string{expired.ID, pending.ID, terminal.ID}; len(gotIDs) != len(want) || gotIDs[0] != want[0] || gotIDs[1] != want[1] || gotIDs[2] != want[2] {
				t.Fatalf("expired reservation IDs = %v, want %v", gotIDs, want)
			}
		})
	}
}

func TestResolveExpiredGenerationUsageReservationAtomicallyBlocksAndClears(t *testing.T) {
	t.Parallel()

	for _, factory := range generationUsageReservationRepoFactories(t) {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			repo := factory.new(t)
			ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
			now := time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC)
			task := createProcessingGenerationUsageReservation(t, repo, ctx, "generation-usage-resolve", now.Add(-time.Minute))
			nextRetryAt := now
			block := &listingkit.RetryableBlock{
				ReasonCode:           "generation_usage_worker_interrupted",
				ReasonMessage:        "worker interrupted",
				BlockedAt:            now,
				NextRetryAt:          &nextRetryAt,
				MaxAutoRetryAttempts: 8,
				RecoveryScope:        "task",
				AutoResumeEnabled:    true,
			}
			if err := repo.ResolveExpiredGenerationUsageReservation(ctx, task.ID, core.TaskStatusProcessing, now, block, "worker interrupted", true); err != nil {
				t.Fatalf("ResolveExpiredGenerationUsageReservation() error = %v", err)
			}

			got, err := repo.GetTask(ctx, task.ID)
			if err != nil {
				t.Fatalf("GetTask() error = %v", err)
			}
			if got.Status != core.TaskStatusBlockedRetryable || got.RetryableBlock == nil || got.RetryableBlock.ReasonCode != block.ReasonCode {
				t.Fatalf("resolved task = %#v, want blocked task with retryable block", got)
			}
			if got.GenerationUsageReservationState != "" || got.GenerationUsageReservationLeaseUntil != nil {
				t.Fatalf("reservation after resolve = (%q, %v), want empty", got.GenerationUsageReservationState, got.GenerationUsageReservationLeaseUntil)
			}
		})
	}
}

func TestResolveExpiredGenerationUsageReservationRejectsRenewedLease(t *testing.T) {
	t.Parallel()

	for _, factory := range generationUsageReservationRepoFactories(t) {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			repo := factory.new(t)
			ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
			now := time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC)
			task := createProcessingGenerationUsageReservation(t, repo, ctx, "generation-usage-renewed-before-claim", now.Add(-time.Minute))
			if err := repo.RenewGenerationUsageReservation(ctx, task.ID, now.Add(time.Minute)); err != nil {
				t.Fatalf("RenewGenerationUsageReservation() error = %v", err)
			}

			err := repo.ResolveExpiredGenerationUsageReservation(ctx, task.ID, core.TaskStatusProcessing, now, &listingkit.RetryableBlock{ReasonCode: "generation_usage_reconciliation_pending", AutoResumeEnabled: false}, "requires reconciliation", false)
			if !errors.Is(err, core.ErrTaskNotRecoverable) {
				t.Fatalf("ResolveExpiredGenerationUsageReservation() error = %v, want ErrTaskNotRecoverable", err)
			}
			got, err := repo.GetTask(ctx, task.ID)
			if err != nil {
				t.Fatalf("GetTask() error = %v", err)
			}
			if got.Status != core.TaskStatusProcessing || got.GenerationUsageReservationLeaseUntil == nil || !got.GenerationUsageReservationLeaseUntil.After(now) {
				t.Fatalf("task after rejected claim = %#v, want still-processing renewed lease", got)
			}
		})
	}
}

func TestResolveExpiredTerminalGenerationUsageReservationFencesConcurrentSettlement(t *testing.T) {
	t.Parallel()

	for _, factory := range generationUsageReservationRepoFactories(t) {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			repo := factory.new(t)
			ctx := context.Background()
			now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
			leaseUntil := now.Add(-time.Minute)
			task := retryableTaskFixture("expired-terminal-generation-usage", now.Add(-time.Hour))
			task.Status = core.TaskStatusCompleted
			task.Result = &listingkit.ListingKitResult{Status: string(core.TaskStatusCompleted)}
			task.GenerationUsageReservationState = listingkit.GenerationUsageReservationStateReserved
			task.GenerationUsageReservationLeaseUntil = &leaseUntil
			if err := repo.CreateTask(ctx, task); err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}

			if err := repo.ClearGenerationUsageReservation(ctx, task.ID); err != nil {
				t.Fatalf("concurrent ClearGenerationUsageReservation() error = %v", err)
			}
			err := repo.ResolveExpiredGenerationUsageReservation(ctx, task.ID, core.TaskStatusCompleted, now, &listingkit.RetryableBlock{ReasonCode: "usage_commit_pending"}, "usage settlement pending", false)
			if !errors.Is(err, core.ErrTaskNotRecoverable) {
				t.Fatalf("ResolveExpiredGenerationUsageReservation() error = %v, want ErrTaskNotRecoverable after concurrent settlement", err)
			}
			got, err := repo.GetTask(ctx, task.ID)
			if err != nil {
				t.Fatalf("GetTask() error = %v", err)
			}
			if got.Status != core.TaskStatusCompleted || got.RetryableBlock != nil || got.GenerationUsageReservationState != "" || got.GenerationUsageReservationLeaseUntil != nil {
				t.Fatalf("concurrently settled task = %#v, want terminal task without a stale commit block", got)
			}
		})
	}
}

type generationUsageReservationRepoFactory struct {
	name string
	new  func(t *testing.T) generationUsageReservationTestRepository
}

type generationUsageReservationTestRepository interface {
	listingkit.Repository
	listingskitGenerationUsageReservationRepository
}

type listingskitGenerationUsageReservationRepository interface {
	BeginGenerationUsageReservation(context.Context, string, time.Time) error
	MarkGenerationUsageReserved(context.Context, string, time.Time) error
	RenewGenerationUsageReservation(context.Context, string, time.Time) error
	ClearGenerationUsageReservation(context.Context, string) error
	ListExpiredGenerationUsageReservations(context.Context, time.Time, int) ([]listingkit.Task, error)
	ResolveExpiredGenerationUsageReservation(context.Context, string, core.TaskStatus, time.Time, *listingkit.RetryableBlock, string, bool) error
}

func generationUsageReservationRepoFactories(t *testing.T) []generationUsageReservationRepoFactory {
	t.Helper()
	return []generationUsageReservationRepoFactory{
		{
			name: "gorm",
			new: func(t *testing.T) generationUsageReservationTestRepository {
				t.Helper()
				repo, ok := retryableTaskRepoFactories(t)[0].new(t).(generationUsageReservationTestRepository)
				if !ok {
					t.Fatal("gorm repository does not implement generation usage reservation operations")
				}
				return repo
			},
		},
		{
			name: "memory",
			new: func(t *testing.T) generationUsageReservationTestRepository {
				t.Helper()
				repo, ok := retryableTaskRepoFactories(t)[1].new(t).(generationUsageReservationTestRepository)
				if !ok {
					t.Fatal("memory repository does not implement generation usage reservation operations")
				}
				return repo
			},
		},
	}
}

func createProcessingGenerationUsageReservation(t *testing.T, repo generationUsageReservationTestRepository, ctx context.Context, taskID string, leaseUntil time.Time) *listingkit.Task {
	t.Helper()
	task := retryableTaskFixture(taskID, leaseUntil.Add(-time.Hour))
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask(%s) error = %v", taskID, err)
	}
	if err := repo.MarkProcessing(ctx, task.ID); err != nil {
		t.Fatalf("MarkProcessing(%s) error = %v", taskID, err)
	}
	if err := repo.BeginGenerationUsageReservation(ctx, task.ID, leaseUntil); err != nil {
		t.Fatalf("BeginGenerationUsageReservation(%s) error = %v", taskID, err)
	}
	if err := repo.MarkGenerationUsageReserved(ctx, task.ID, leaseUntil); err != nil {
		t.Fatalf("MarkGenerationUsageReserved(%s) error = %v", taskID, err)
	}
	return task
}
