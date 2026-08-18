package store

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
)

func TestTaskRepositoryUsageSettlementResolutionRetainsTerminalResult(t *testing.T) {
	for _, factory := range []struct {
		name string
		new  func(*testing.T) listingkit.Repository
	}{
		{name: "memory", new: func(*testing.T) listingkit.Repository { return NewMemTaskRepository() }},
		{name: "gorm", new: func(t *testing.T) listingkit.Repository {
			t.Helper()
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				t.Fatalf("open db: %v", err)
			}
			if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
				t.Fatalf("auto migrate: %v", err)
			}
			return NewTaskRepository(db)
		}},
	} {
		t.Run(factory.name, func(t *testing.T) {
			repo := factory.new(t)
			leaseUntil := time.Now().UTC().Add(time.Hour)
			task := &listingkit.Task{ID: "task-usage-settlement", TenantID: "tenant-17", Status: core.TaskStatusCompleted, Result: &listingkit.ListingKitResult{Status: string(core.TaskStatusCompleted)}, GenerationUsageReservationState: listingkit.GenerationUsageReservationStateReserved, GenerationUsageReservationLeaseUntil: &leaseUntil, CreatedAt: time.Now().UTC()}
			if err := repo.CreateTask(context.Background(), task); err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			if err := repo.MarkBlockedRetryable(context.Background(), task.ID, &listingkit.RetryableBlock{ReasonCode: "usage_commit_pending"}, "usage settlement pending"); err != nil {
				t.Fatalf("MarkBlockedRetryable() error = %v", err)
			}
			settlementRepo, ok := repo.(listingkit.UsageSettlementRepository)
			if !ok {
				t.Fatal("repository does not implement UsageSettlementRepository")
			}
			if err := settlementRepo.ResolveUsageSettlement(context.Background(), task.ID); err != nil {
				t.Fatalf("ResolveUsageSettlement() error = %v", err)
			}
			got, err := repo.GetTask(context.Background(), task.ID)
			if err != nil {
				t.Fatalf("GetTask() error = %v", err)
			}
			if got.Status != core.TaskStatusCompleted || got.Result == nil || got.Result.Status != string(core.TaskStatusCompleted) || got.RetryableBlock != nil || got.Error != "" || got.GenerationUsageReservationState != "" || got.GenerationUsageReservationLeaseUntil != nil {
				t.Fatalf("resolved task = %#v, want completed result with cleared block and reservation intent", got)
			}
		})
	}
}

func TestTaskRepositoryUsageSettlementResolutionRestoresNeedsReviewReason(t *testing.T) {
	for _, factory := range []struct {
		name string
		new  func(*testing.T) listingkit.Repository
	}{
		{name: "memory", new: func(*testing.T) listingkit.Repository { return NewMemTaskRepository() }},
		{name: "gorm", new: func(t *testing.T) listingkit.Repository {
			t.Helper()
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				t.Fatalf("open db: %v", err)
			}
			if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
				t.Fatalf("auto migrate: %v", err)
			}
			return NewTaskRepository(db)
		}},
	} {
		t.Run(factory.name, func(t *testing.T) {
			repo := factory.new(t)
			leaseUntil := time.Now().UTC().Add(time.Hour)
			result := &listingkit.ListingKitResult{Status: string(core.TaskStatusNeedsReview), ReviewReasons: []string{"title requires review"}}
			task := &listingkit.Task{ID: "task-usage-settlement-review", TenantID: "tenant-17", Status: core.TaskStatusNeedsReview, Result: result, GenerationUsageReservationState: listingkit.GenerationUsageReservationStateReserved, GenerationUsageReservationLeaseUntil: &leaseUntil, CreatedAt: time.Now().UTC()}
			if err := repo.CreateTask(context.Background(), task); err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			if err := repo.MarkBlockedRetryable(context.Background(), task.ID, &listingkit.RetryableBlock{ReasonCode: "usage_commit_pending"}, "usage settlement pending"); err != nil {
				t.Fatalf("MarkBlockedRetryable() error = %v", err)
			}
			settlementRepo := repo.(listingkit.UsageSettlementRepository)
			if err := settlementRepo.ResolveUsageSettlement(context.Background(), task.ID); err != nil {
				t.Fatalf("ResolveUsageSettlement() error = %v", err)
			}
			got, err := repo.GetTask(context.Background(), task.ID)
			if err != nil {
				t.Fatalf("GetTask() error = %v", err)
			}
			if got.Status != core.TaskStatusNeedsReview || got.Error != "title requires review" || got.RetryableBlock != nil || got.GenerationUsageReservationState != "" || got.GenerationUsageReservationLeaseUntil != nil {
				t.Fatalf("resolved task = %#v, want restored review reason and cleared settlement state", got)
			}
		})
	}
}

func TestTaskRepositoryTerminalTransitionsClearRecoveredRetryableBlock(t *testing.T) {
	for _, factory := range []struct {
		name string
		new  func(*testing.T) listingkit.Repository
	}{
		{name: "memory", new: func(*testing.T) listingkit.Repository { return NewMemTaskRepository() }},
		{name: "gorm", new: func(t *testing.T) listingkit.Repository {
			t.Helper()
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				t.Fatalf("open db: %v", err)
			}
			if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
				t.Fatalf("auto migrate: %v", err)
			}
			return NewTaskRepository(db)
		}},
	} {
		t.Run(factory.name, func(t *testing.T) {
			repo := factory.new(t)
			ctx := context.Background()
			for _, terminal := range []struct {
				name  string
				apply func(listingkit.Repository, string) error
			}{
				{name: "completed", apply: func(repo listingkit.Repository, taskID string) error {
					return repo.MarkCompleted(ctx, taskID, &listingkit.ListingKitResult{Status: string(core.TaskStatusCompleted)})
				}},
				{name: "needs_review", apply: func(repo listingkit.Repository, taskID string) error {
					return repo.MarkNeedsReview(ctx, taskID, &listingkit.ListingKitResult{Status: string(core.TaskStatusNeedsReview)}, "manual review")
				}},
			} {
				t.Run(terminal.name, func(t *testing.T) {
					task := &listingkit.Task{ID: "task-terminal-clears-" + terminal.name, TenantID: "tenant-17", Status: core.TaskStatusPending, CreatedAt: time.Now().UTC()}
					if err := repo.CreateTask(ctx, task); err != nil {
						t.Fatalf("CreateTask() error = %v", err)
					}
					if err := repo.MarkBlockedRetryable(ctx, task.ID, &listingkit.RetryableBlock{ReasonCode: "provider_retry", AutoResumeEnabled: true}, "retry provider"); err != nil {
						t.Fatalf("MarkBlockedRetryable() error = %v", err)
					}
					if err := terminal.apply(repo, task.ID); err != nil {
						t.Fatalf("terminal transition error = %v", err)
					}
					got, err := repo.GetTask(ctx, task.ID)
					if err != nil {
						t.Fatalf("GetTask() error = %v", err)
					}
					if got.RetryableBlock != nil {
						t.Fatalf("terminal task = %#v, want cleared retryable block", got)
					}
				})
			}
		})
	}
}

func TestTaskRepositoryGenerationUsageReleaseResolutionIsAtomic(t *testing.T) {
	for _, factory := range []struct {
		name string
		new  func(*testing.T) listingkit.Repository
	}{
		{name: "memory", new: func(*testing.T) listingkit.Repository { return NewMemTaskRepository() }},
		{name: "gorm", new: func(t *testing.T) listingkit.Repository {
			t.Helper()
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				t.Fatalf("open db: %v", err)
			}
			if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
				t.Fatalf("auto migrate: %v", err)
			}
			return NewTaskRepository(db)
		}},
	} {
		t.Run(factory.name, func(t *testing.T) {
			repo := factory.new(t)
			ctx := context.Background()
			leaseUntil := time.Now().UTC().Add(time.Hour)
			task := &listingkit.Task{ID: "task-release-resolution", TenantID: "tenant-17", Status: core.TaskStatusProcessing, GenerationUsageReservationState: listingkit.GenerationUsageReservationStateReserved, GenerationUsageReservationLeaseUntil: &leaseUntil, CreatedAt: time.Now().UTC()}
			if err := repo.CreateTask(ctx, task); err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			recovery, ok := repo.(listingkit.GenerationUsageReleaseRecoveryRepository)
			if !ok {
				t.Fatal("repository does not implement GenerationUsageReleaseRecoveryRepository")
			}
			block := &listingkit.RetryableBlock{ReasonCode: "usage_release_pending", ReasonMessage: "usage release is pending", TerminalError: "provider rejected listing generation"}
			if err := recovery.PrepareGenerationUsageRelease(ctx, task.ID, block, block.ReasonMessage, &listingkit.ListingKitResult{Status: string(core.TaskStatusFailed)}); err != nil {
				t.Fatalf("PrepareGenerationUsageRelease() error = %v", err)
			}
			if err := recovery.ResolveGenerationUsageRelease(ctx, task.ID, block.TerminalError); err != nil {
				t.Fatalf("ResolveGenerationUsageRelease() error = %v", err)
			}
			got, err := repo.GetTask(ctx, task.ID)
			if err != nil {
				t.Fatalf("GetTask() error = %v", err)
			}
			if got.Status != core.TaskStatusFailed || got.Error != block.TerminalError || got.RetryableBlock != nil || got.GenerationUsageReservationState != "" || got.GenerationUsageReservationLeaseUntil != nil || got.Result == nil {
				t.Fatalf("resolved task = %#v, want atomically finalized release saga", got)
			}
		})
	}
}

func TestTaskRepositoryFinalizesRejectedUsageAdmissionAtomically(t *testing.T) {
	for _, factory := range []struct {
		name string
		new  func(*testing.T) listingkit.Repository
	}{
		{name: "memory", new: func(*testing.T) listingkit.Repository { return NewMemTaskRepository() }},
		{name: "gorm", new: func(t *testing.T) listingkit.Repository {
			t.Helper()
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				t.Fatalf("open db: %v", err)
			}
			if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
				t.Fatalf("auto migrate: %v", err)
			}
			return NewTaskRepository(db)
		}},
	} {
		t.Run(factory.name, func(t *testing.T) {
			repo := factory.new(t)
			ctx := context.Background()
			leaseUntil := time.Now().UTC().Add(time.Hour)
			task := &listingkit.Task{ID: "task-admission-rejected", TenantID: "tenant-17", Status: core.TaskStatusProcessing, GenerationUsageReservationState: listingkit.GenerationUsageReservationStatePending, GenerationUsageReservationLeaseUntil: &leaseUntil, CreatedAt: time.Now().UTC()}
			if err := repo.CreateTask(ctx, task); err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			admission, ok := repo.(listingkit.GenerationUsageAdmissionRepository)
			if !ok {
				t.Fatal("repository does not implement GenerationUsageAdmissionRepository")
			}
			block := &listingkit.RetryableBlock{ReasonCode: "terminal_persistence_pending", TerminalError: "listingkit generation quota exceeded"}
			if err := admission.FinalizeGenerationUsageAdmission(ctx, task.ID, core.TaskStatusBlockedRetryable, block, "terminal persistence pending"); err != nil {
				t.Fatalf("FinalizeGenerationUsageAdmission() error = %v", err)
			}
			got, err := repo.GetTask(ctx, task.ID)
			if err != nil {
				t.Fatalf("GetTask() error = %v", err)
			}
			if got.Status != core.TaskStatusBlockedRetryable || got.RetryableBlock == nil || got.RetryableBlock.TerminalError != block.TerminalError || got.GenerationUsageReservationState != "" || got.GenerationUsageReservationLeaseUntil != nil {
				t.Fatalf("finalized admission task = %#v, want terminal block with cleared intent", got)
			}
		})
	}
}
