package listingkit

import (
	"context"
	"testing"
	"time"
)

func TestGenerationUsageIdentityUsesStableTaskKey(t *testing.T) {
	t.Parallel()

	got := generationUsageIdentity(" task-42 ", "tenant-17", time.Date(2026, 8, 14, 2, 3, 4, 0, time.UTC))
	if got.TenantID != "tenant-17" || got.TaskID != "task-42" {
		t.Fatalf("identity = %#v, want normalized tenant/task", got)
	}
	if got.IdempotencyKey != "listingkit:generation:task-42" {
		t.Fatalf("idempotency key = %q, want stable task key", got.IdempotencyKey)
	}
	if got.SourceType != "listingkit_generation" || got.SourceID != "task-42" {
		t.Fatalf("source = (%q, %q), want ListingKit generation source", got.SourceType, got.SourceID)
	}
	if got.ModuleCode != "studio" || got.Metric != "studio_design_jobs_succeeded" || got.Quantity != 1 {
		t.Fatalf("usage fact = %#v, want canonical studio generation fact", got)
	}
	if !got.OccurredAt.Equal(time.Date(2026, 8, 14, 2, 3, 4, 0, time.UTC)) {
		t.Fatalf("occurred_at = %v, want supplied time", got.OccurredAt)
	}
}

func TestGenerationUsageSettlementPortIsOptional(t *testing.T) {
	t.Parallel()

	var settlement GenerationUsageSettlement
	if settlement != nil {
		t.Fatal("zero GenerationUsageSettlement = non-nil")
	}
	_ = context.Background()
}
