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
