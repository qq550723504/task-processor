package store

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
)

func TestMemTaskRepositoryUsageSettlementResolutionRetainsTerminalResult(t *testing.T) {
	t.Parallel()

	repo := NewMemTaskRepository()
	task := &listingkit.Task{ID: "task-usage-settlement", TenantID: "tenant-17", Status: core.TaskStatusCompleted, Result: &listingkit.ListingKitResult{Status: string(core.TaskStatusCompleted)}, CreatedAt: time.Now().UTC()}
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
	if got.Status != core.TaskStatusCompleted || got.Result == nil || got.Result.Status != string(core.TaskStatusCompleted) || got.RetryableBlock != nil || got.Error != "" {
		t.Fatalf("resolved task = %#v, want completed result with cleared block", got)
	}
}
