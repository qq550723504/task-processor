package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestGormStudioBatchRepositoryRejectsStaleBackgroundRemovalSourceAttempt(t *testing.T) {
	t.Parallel()

	db := openStudioBatchSQLiteForTest(t)
	if err := AutoMigrateStudioBatchRepository(db); err != nil {
		t.Fatalf("AutoMigrateStudioBatchRepository() error = %v", err)
	}
	repo := NewGormStudioBatchRepository(db)
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()
	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), newStudioBatchDesignsForTest("batch-1", "item-1", now)); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	stale := &StudioMaterializedDesignRecord{
		ID:                        "design-1",
		BatchID:                   "batch-1",
		ItemID:                    "item-1",
		SourceAttemptID:           "attempt-2",
		ImageURL:                  "https://cdn.example.test/stale.png",
		OriginalImageURL:          "https://cdn.example.test/design-1-original.png",
		TransparentBackgroundMode: StudioTransparencyModeRemoval,
		BackgroundRemovalStatus:   StudioBackgroundRemovalStatusPending,
		UpdatedAt:                 now.Add(time.Minute),
	}
	claimed, err := repo.ClaimStudioMaterializedDesignBackgroundRemoval(ctx, stale)
	if err != nil {
		t.Fatalf("ClaimStudioMaterializedDesignBackgroundRemoval() error = %v", err)
	}
	if claimed {
		t.Fatal("ClaimStudioMaterializedDesignBackgroundRemoval() claimed stale source attempt")
	}

	stale.BackgroundRemovalStatus = StudioBackgroundRemovalStatusSucceeded
	if err := repo.UpdateStudioMaterializedDesignBackgroundRemoval(ctx, stale); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("UpdateStudioMaterializedDesignBackgroundRemoval() error = %v, want record not found", err)
	}
	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	design := detail.DesignsByItem["item-1"][0]
	if design.SourceAttemptID != "attempt-1" || design.BackgroundRemovalStatus == StudioBackgroundRemovalStatusPending || design.ImageURL != "https://cdn.example.com/design-1.png" {
		t.Fatalf("design after stale update = %+v, want original source attempt and image", design)
	}
}
