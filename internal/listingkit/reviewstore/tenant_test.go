package reviewstore

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/shared/tenantctx"
)

func TestMemRepositoryScopesReviewsByTenant(t *testing.T) {
	repo := NewMemRepository()
	ctxA := tenantctx.WithTenantID(context.Background(), "tenant-a")
	ctxB := tenantctx.WithTenantID(context.Background(), "tenant-b")

	if err := repo.SaveReview(ctxA, &ReviewRecord{TaskID: "task-1", Platform: "shein", Slot: "main", Capability: "image", Decision: "approve", Status: "approved", ReviewedAt: time.Now()}); err != nil {
		t.Fatalf("SaveReview tenant-a: %v", err)
	}
	if err := repo.SaveReview(ctxB, &ReviewRecord{TaskID: "task-1", Platform: "shein", Slot: "main", Capability: "image", Decision: "reject", Status: "rejected", ReviewedAt: time.Now()}); err != nil {
		t.Fatalf("SaveReview tenant-b: %v", err)
	}

	reviews, err := repo.ListReviews(ctxA, "task-1")
	if err != nil {
		t.Fatalf("ListReviews tenant-a: %v", err)
	}
	if len(reviews) != 1 || reviews[0].TenantID != "tenant-a" || reviews[0].Decision != "approve" {
		t.Fatalf("tenant-a reviews = %#v", reviews)
	}
}
