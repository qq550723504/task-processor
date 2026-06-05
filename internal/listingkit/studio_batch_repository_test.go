package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMemStudioBatchRepositoryRejectsOrphanAttemptAndDesign(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")

	err := repo.CreateStudioBatchGraph(ctx, &StudioBatchRecord{
		ID:     "batch-1",
		Status: StudioBatchStatusDraft,
	}, []StudioBatchItemRecord{{
		ID:      "item-1",
		BatchID: "batch-1",
		Status:  StudioBatchItemStatusPending,
	}}, []StudioGenerationAttemptRecord{{
		ID:        "attempt-orphan",
		ItemID:    "missing-item",
		AttemptNo: 1,
		Status:    StudioGenerationAttemptStatusQueued,
	}}, nil)
	if !errors.Is(err, ErrStudioBatchUnknownItemReference) {
		t.Fatalf("CreateStudioBatchGraph(orphan attempt) error = %v, want ErrStudioBatchUnknownItemReference", err)
	}

	err = repo.CreateStudioBatchGraph(ctx, &StudioBatchRecord{
		ID:     "batch-2",
		Status: StudioBatchStatusDraft,
	}, []StudioBatchItemRecord{{
		ID:      "item-2",
		BatchID: "batch-2",
		Status:  StudioBatchItemStatusPending,
	}}, nil, []StudioMaterializedDesignRecord{{
		ID:       "design-orphan",
		BatchID:  "batch-2",
		ItemID:   "missing-item",
		ImageURL: "https://cdn.example.com/orphan.png",
	}})
	if !errors.Is(err, ErrStudioBatchUnknownItemReference) {
		t.Fatalf("CreateStudioBatchGraph(orphan design) error = %v, want ErrStudioBatchUnknownItemReference", err)
	}
}

func TestMemStudioBatchRepositoryCreatesDetailGraph(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), newStudioBatchDesignsForTest("batch-1", "item-1", now)); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if detail.Batch == nil || detail.Batch.ID != "batch-1" {
		t.Fatalf("detail.Batch = %+v, want batch-1", detail.Batch)
	}
	if len(detail.Items) != 1 || detail.Items[0].ID != "item-1" {
		t.Fatalf("detail.Items = %+v, want item-1", detail.Items)
	}
	if len(detail.AttemptsByItem["item-1"]) != 1 || detail.AttemptsByItem["item-1"][0].ID != "attempt-1" {
		t.Fatalf("detail.AttemptsByItem = %+v, want attempt-1", detail.AttemptsByItem)
	}
	if len(detail.DesignsByItem["item-1"]) != 1 || detail.DesignsByItem["item-1"][0].ID != "design-1" {
		t.Fatalf("detail.DesignsByItem = %+v, want design-1", detail.DesignsByItem)
	}
}

func TestMemStudioBatchRepositoryGetBatchAndItemAndListDesignsAndUpdate(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-2",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			TargetGroupKey:  "size:1200x1200",
			ImageURL:        "https://cdn.example.com/design-2.png",
			SortOrder:       2,
			CreatedAt:       now.Add(2 * time.Second),
			UpdatedAt:       now.Add(2 * time.Second),
		},
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			TargetGroupKey:  "size:1200x1200",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			SortOrder:       1,
			CreatedAt:       now.Add(time.Second),
			UpdatedAt:       now.Add(time.Second),
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	batch, err := repo.GetStudioBatch(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatch() error = %v", err)
	}
	if batch.SheinStoreID != 9001 {
		t.Fatalf("GetStudioBatch().SheinStoreID = %d, want 9001", batch.SheinStoreID)
	}

	item, err := repo.GetStudioBatchItem(ctx, "item-1")
	if err != nil {
		t.Fatalf("GetStudioBatchItem() error = %v", err)
	}
	if item.BatchID != "batch-1" || item.SelectionCount != 3 {
		t.Fatalf("GetStudioBatchItem() = %+v, want batch-1 selection_count=3", item)
	}

	designs, err := repo.ListStudioMaterializedDesignsByIDs(ctx, "batch-1", []string{"design-2", "design-1"})
	if err != nil {
		t.Fatalf("ListStudioMaterializedDesignsByIDs() error = %v", err)
	}
	if len(designs) != 2 || designs[0].ID != "design-1" || designs[0].ReviewStatus != StudioMaterializedDesignReviewStatusApproved || designs[1].ID != "design-2" || designs[1].ReviewStatus != StudioMaterializedDesignReviewStatusApproved {
		t.Fatalf("ListStudioMaterializedDesignsByIDs() = %+v, want ordered review-status designs", designs)
	}

	batch.Status = StudioBatchStatusGenerating
	batch.UpdatedAt = now.Add(5 * time.Second)
	if err := repo.UpdateStudioBatch(ctx, batch); err != nil {
		t.Fatalf("UpdateStudioBatch() error = %v", err)
	}

	updatedBatch, err := repo.GetStudioBatch(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatch(updated) error = %v", err)
	}
	if updatedBatch.Status != StudioBatchStatusGenerating {
		t.Fatalf("updated batch status = %q, want generating", updatedBatch.Status)
	}
}

func TestMemStudioBatchRepositoryRejectsOwnershipCorruptionOnUpdates(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), newStudioBatchDesignsForTest("batch-1", "item-1", now)); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	if err := repo.UpdateStudioBatchItem(ctx, &StudioBatchItemRecord{
		ID:      "item-1",
		BatchID: "batch-2",
		Status:  StudioBatchItemStatusGenerating,
	}); !errors.Is(err, ErrStudioBatchOwnershipConflict) {
		t.Fatalf("UpdateStudioBatchItem(conflicting batch) error = %v, want ErrStudioBatchOwnershipConflict", err)
	}
	if err := repo.UpdateStudioGenerationAttempt(ctx, &StudioGenerationAttemptRecord{
		ID:        "attempt-1",
		ItemID:    "item-2",
		BatchID:   "batch-2",
		AttemptNo: 2,
		Status:    StudioGenerationAttemptStatusRunning,
	}); !errors.Is(err, ErrStudioBatchOwnershipConflict) {
		t.Fatalf("UpdateStudioGenerationAttempt(conflicting ownership) error = %v, want ErrStudioBatchOwnershipConflict", err)
	}
	if err := repo.UpdateStudioMaterializedDesign(ctx, &StudioMaterializedDesignRecord{
		ID:              "design-1",
		BatchID:         "batch-2",
		ItemID:          "item-2",
		SourceAttemptID: "attempt-9",
		ReviewStatus:    StudioMaterializedDesignReviewStatusRejected,
	}); !errors.Is(err, ErrStudioBatchOwnershipConflict) {
		t.Fatalf("UpdateStudioMaterializedDesign(conflicting ownership) error = %v, want ErrStudioBatchOwnershipConflict", err)
	}
	if err := repo.UpdateStudioBatchItem(ctx, &StudioBatchItemRecord{
		ID:       "item-1",
		BatchID:  "batch-1",
		TenantID: "tenant-b",
		UserID:   "user-b",
		Status:   StudioBatchItemStatusGenerating,
	}); !errors.Is(err, ErrStudioBatchOwnershipConflict) {
		t.Fatalf("UpdateStudioBatchItem(conflicting owner scope) error = %v, want ErrStudioBatchOwnershipConflict", err)
	}
	if err := repo.UpdateStudioGenerationAttempt(ctx, &StudioGenerationAttemptRecord{
		ID:        "attempt-1",
		ItemID:    "item-1",
		BatchID:   "batch-1",
		TenantID:  "tenant-b",
		UserID:    "user-b",
		AttemptNo: 2,
		Status:    StudioGenerationAttemptStatusRunning,
	}); !errors.Is(err, ErrStudioBatchOwnershipConflict) {
		t.Fatalf("UpdateStudioGenerationAttempt(conflicting owner scope) error = %v, want ErrStudioBatchOwnershipConflict", err)
	}
	if err := repo.UpdateStudioMaterializedDesign(ctx, &StudioMaterializedDesignRecord{
		ID:              "design-1",
		BatchID:         "batch-1",
		ItemID:          "item-1",
		SourceAttemptID: "attempt-1",
		TenantID:        "tenant-b",
		UserID:          "user-b",
		ReviewStatus:    StudioMaterializedDesignReviewStatusRejected,
	}); !errors.Is(err, ErrStudioBatchOwnershipConflict) {
		t.Fatalf("UpdateStudioMaterializedDesign(conflicting owner scope) error = %v, want ErrStudioBatchOwnershipConflict", err)
	}
}

func TestMemStudioBatchRepositoryScopesByTenantAndOwner(t *testing.T) {
	restore := SetOwnerScopeRequiredForTesting(true)
	defer restore()

	repo := NewMemStudioBatchRepository()
	baseCtx := WithTenantID(context.Background(), "tenant-a")
	ctxUserA := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-a"})
	ctxUserB := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-b"})
	ctxTenantB := WithTenantID(context.Background(), "tenant-b")

	if err := repo.CreateStudioBatchGraph(ctxUserA, &StudioBatchRecord{
		ID:     "batch-1",
		Status: StudioBatchStatusDraft,
	}, nil, nil, nil); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	if _, err := repo.GetStudioBatchDetail(ctxUserA, "batch-1"); err != nil {
		t.Fatalf("same-user GetStudioBatchDetail() error = %v", err)
	}
	if _, err := repo.GetStudioBatchDetail(ctxUserB, "batch-1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-user GetStudioBatchDetail() error = %v, want record not found", err)
	}
	if _, err := repo.GetStudioBatchDetail(ctxTenantB, "batch-1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant GetStudioBatchDetail() error = %v, want record not found", err)
	}
	if _, err := repo.GetStudioBatch(ctxUserB, "batch-1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-user GetStudioBatch() error = %v, want record not found", err)
	}
	if _, err := repo.GetStudioBatchItem(ctxUserB, "item-1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-user GetStudioBatchItem() error = %v, want record not found", err)
	}
}

func TestGormStudioBatchRepositoryCreatesDetailGraph(t *testing.T) {
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

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if detail.Batch == nil || detail.Batch.ID != "batch-1" {
		t.Fatalf("detail.Batch = %+v, want batch-1", detail.Batch)
	}
	if len(detail.Items) != 1 || detail.Items[0].ID != "item-1" {
		t.Fatalf("detail.Items = %+v, want item-1", detail.Items)
	}
	if len(detail.AttemptsByItem["item-1"]) != 1 || detail.AttemptsByItem["item-1"][0].ID != "attempt-1" {
		t.Fatalf("detail.AttemptsByItem = %+v, want attempt-1", detail.AttemptsByItem)
	}
	if len(detail.DesignsByItem["item-1"]) != 1 || detail.DesignsByItem["item-1"][0].ID != "design-1" {
		t.Fatalf("detail.DesignsByItem = %+v, want design-1", detail.DesignsByItem)
	}
}

func TestGormStudioBatchRepositoryRejectsOrphanAttemptAndDesign(t *testing.T) {
	t.Parallel()

	db := openStudioBatchSQLiteForTest(t)
	if err := AutoMigrateStudioBatchRepository(db); err != nil {
		t.Fatalf("AutoMigrateStudioBatchRepository() error = %v", err)
	}

	repo := NewGormStudioBatchRepository(db)
	ctx := WithTenantID(context.Background(), "tenant-a")

	err := repo.CreateStudioBatchGraph(ctx, &StudioBatchRecord{
		ID:     "batch-1",
		Status: StudioBatchStatusDraft,
	}, []StudioBatchItemRecord{{
		ID:      "item-1",
		BatchID: "batch-1",
		Status:  StudioBatchItemStatusPending,
	}}, []StudioGenerationAttemptRecord{{
		ID:        "attempt-orphan",
		ItemID:    "missing-item",
		AttemptNo: 1,
		Status:    StudioGenerationAttemptStatusQueued,
	}}, nil)
	if !errors.Is(err, ErrStudioBatchUnknownItemReference) {
		t.Fatalf("CreateStudioBatchGraph(orphan attempt) error = %v, want ErrStudioBatchUnknownItemReference", err)
	}

	err = repo.CreateStudioBatchGraph(ctx, &StudioBatchRecord{
		ID:     "batch-2",
		Status: StudioBatchStatusDraft,
	}, []StudioBatchItemRecord{{
		ID:      "item-2",
		BatchID: "batch-2",
		Status:  StudioBatchItemStatusPending,
	}}, nil, []StudioMaterializedDesignRecord{{
		ID:       "design-orphan",
		BatchID:  "batch-2",
		ItemID:   "missing-item",
		ImageURL: "https://cdn.example.com/orphan.png",
	}})
	if !errors.Is(err, ErrStudioBatchUnknownItemReference) {
		t.Fatalf("CreateStudioBatchGraph(orphan design) error = %v, want ErrStudioBatchUnknownItemReference", err)
	}
}

func TestGormStudioBatchRepositoryScopesByTenant(t *testing.T) {
	t.Parallel()

	db := openStudioBatchSQLiteForTest(t)
	if err := AutoMigrateStudioBatchRepository(db); err != nil {
		t.Fatalf("AutoMigrateStudioBatchRepository() error = %v", err)
	}

	repo := NewGormStudioBatchRepository(db)
	ctxA := WithTenantID(context.Background(), "tenant-a")
	ctxB := WithTenantID(context.Background(), "tenant-b")

	if err := repo.CreateStudioBatchGraph(ctxA, &StudioBatchRecord{
		ID:     "batch-1",
		Status: StudioBatchStatusDraft,
	}, nil, nil, nil); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	if _, err := repo.GetStudioBatchDetail(ctxB, "batch-1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetStudioBatchDetail() error = %v, want record not found", err)
	}
}

func TestGormStudioBatchRepositoryOwnerScopeAndPublicAccessorsAndUpdate(t *testing.T) {
	restore := SetOwnerScopeRequiredForTesting(true)
	defer restore()

	db := openStudioBatchSQLiteForTest(t)
	if err := AutoMigrateStudioBatchRepository(db); err != nil {
		t.Fatalf("AutoMigrateStudioBatchRepository() error = %v", err)
	}

	repo := NewGormStudioBatchRepository(db)
	baseCtx := WithTenantID(context.Background(), "tenant-a")
	ctxUserA := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-a"})
	ctxUserB := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-b"})
	now := time.Now().UTC()

	if err := repo.CreateStudioBatchGraph(ctxUserA, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-2",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			TargetGroupKey:  "size:1200x1200",
			ImageURL:        "https://cdn.example.com/design-2.png",
			SortOrder:       2,
			CreatedAt:       now.Add(2 * time.Second),
			UpdatedAt:       now.Add(2 * time.Second),
		},
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			TargetGroupKey:  "size:1200x1200",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			SortOrder:       1,
			CreatedAt:       now.Add(time.Second),
			UpdatedAt:       now.Add(time.Second),
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	batch, err := repo.GetStudioBatch(ctxUserA, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatch() error = %v", err)
	}
	if batch.SheinStoreID != 9001 {
		t.Fatalf("GetStudioBatch().SheinStoreID = %d, want 9001", batch.SheinStoreID)
	}

	item, err := repo.GetStudioBatchItem(ctxUserA, "item-1")
	if err != nil {
		t.Fatalf("GetStudioBatchItem() error = %v", err)
	}
	if item.UserID != "user-a" {
		t.Fatalf("GetStudioBatchItem().UserID = %q, want user-a", item.UserID)
	}

	designs, err := repo.ListStudioMaterializedDesignsByIDs(ctxUserA, "batch-1", []string{"design-2", "design-1"})
	if err != nil {
		t.Fatalf("ListStudioMaterializedDesignsByIDs() error = %v", err)
	}
	if len(designs) != 2 || designs[0].ID != "design-1" || designs[0].ReviewStatus != StudioMaterializedDesignReviewStatusApproved || designs[1].ID != "design-2" || designs[1].ReviewStatus != StudioMaterializedDesignReviewStatusApproved {
		t.Fatalf("ListStudioMaterializedDesignsByIDs() = %+v, want ordered review-status designs", designs)
	}

	batch.Status = StudioBatchStatusGenerating
	batch.UpdatedAt = now.Add(5 * time.Second)
	if err := repo.UpdateStudioBatch(ctxUserA, batch); err != nil {
		t.Fatalf("UpdateStudioBatch() error = %v", err)
	}

	updatedBatch, err := repo.GetStudioBatch(ctxUserA, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatch(updated) error = %v", err)
	}
	if updatedBatch.Status != StudioBatchStatusGenerating {
		t.Fatalf("updated batch status = %q, want generating", updatedBatch.Status)
	}

	if _, err := repo.GetStudioBatch(ctxUserB, "batch-1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-user GetStudioBatch() error = %v, want record not found", err)
	}
	if _, err := repo.GetStudioBatchItem(ctxUserB, "item-1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-user GetStudioBatchItem() error = %v, want record not found", err)
	}
	if _, err := repo.GetStudioBatchDetail(ctxUserB, "batch-1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-user GetStudioBatchDetail() error = %v, want record not found", err)
	}
	if _, err := repo.ListStudioMaterializedDesignsByIDs(ctxUserB, "batch-1", []string{"design-1"}); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-user ListStudioMaterializedDesignsByIDs() error = %v, want record not found", err)
	}
}

func TestGormStudioBatchRepositoryRejectsOwnershipCorruptionOnUpdates(t *testing.T) {
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

	if err := repo.UpdateStudioBatchItem(ctx, &StudioBatchItemRecord{
		ID:      "item-1",
		BatchID: "batch-2",
		Status:  StudioBatchItemStatusGenerating,
	}); !errors.Is(err, ErrStudioBatchOwnershipConflict) {
		t.Fatalf("UpdateStudioBatchItem(conflicting batch) error = %v, want ErrStudioBatchOwnershipConflict", err)
	}
	if err := repo.UpdateStudioGenerationAttempt(ctx, &StudioGenerationAttemptRecord{
		ID:        "attempt-1",
		ItemID:    "item-2",
		BatchID:   "batch-2",
		AttemptNo: 2,
		Status:    StudioGenerationAttemptStatusRunning,
	}); !errors.Is(err, ErrStudioBatchOwnershipConflict) {
		t.Fatalf("UpdateStudioGenerationAttempt(conflicting ownership) error = %v, want ErrStudioBatchOwnershipConflict", err)
	}
	if err := repo.UpdateStudioMaterializedDesign(ctx, &StudioMaterializedDesignRecord{
		ID:              "design-1",
		BatchID:         "batch-2",
		ItemID:          "item-2",
		SourceAttemptID: "attempt-9",
		ReviewStatus:    StudioMaterializedDesignReviewStatusRejected,
	}); !errors.Is(err, ErrStudioBatchOwnershipConflict) {
		t.Fatalf("UpdateStudioMaterializedDesign(conflicting ownership) error = %v, want ErrStudioBatchOwnershipConflict", err)
	}
}

func TestMemStudioBatchRepositoryReplaceDesignReviewsIsAtomic(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			SortOrder:       0,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "design-2",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-2.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusRejected,
			SortOrder:       1,
			CreatedAt:       now.Add(time.Second),
			UpdatedAt:       now.Add(time.Second),
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	if err := repo.ReplaceStudioMaterializedDesignReviews(ctx, "batch-1", []string{"design-2", "missing-design"}, now.Add(2*time.Second)); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("ReplaceStudioMaterializedDesignReviews() error = %v, want record not found", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if got := detail.DesignsByItem["item-1"][0].ReviewStatus; got != StudioMaterializedDesignReviewStatusApproved {
		t.Fatalf("design-1 review status = %q, want approved", got)
	}
	if got := detail.DesignsByItem["item-1"][1].ReviewStatus; got != StudioMaterializedDesignReviewStatusRejected {
		t.Fatalf("design-2 review status = %q, want rejected", got)
	}
}

func TestGormStudioBatchRepositoryReplaceDesignReviewsIsAtomic(t *testing.T) {
	t.Parallel()

	db := openStudioBatchSQLiteForTest(t)
	if err := AutoMigrateStudioBatchRepository(db); err != nil {
		t.Fatalf("AutoMigrateStudioBatchRepository() error = %v", err)
	}

	repo := NewGormStudioBatchRepository(db)
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			SortOrder:       0,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "design-2",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-2.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusRejected,
			SortOrder:       1,
			CreatedAt:       now.Add(time.Second),
			UpdatedAt:       now.Add(time.Second),
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	if err := repo.ReplaceStudioMaterializedDesignReviews(ctx, "batch-1", []string{"design-2", "missing-design"}, now.Add(2*time.Second)); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("ReplaceStudioMaterializedDesignReviews() error = %v, want record not found", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if got := detail.DesignsByItem["item-1"][0].ReviewStatus; got != StudioMaterializedDesignReviewStatusApproved {
		t.Fatalf("design-1 review status = %q, want approved", got)
	}
	if got := detail.DesignsByItem["item-1"][1].ReviewStatus; got != StudioMaterializedDesignReviewStatusRejected {
		t.Fatalf("design-2 review status = %q, want rejected", got)
	}
}

func openStudioBatchSQLiteForTest(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

func newStudioBatchRecordForTest(batchID string, now time.Time) *StudioBatchRecord {
	return &StudioBatchRecord{
		ID:               batchID,
		Status:           StudioBatchStatusDraft,
		GroupedImageMode: "shared_by_size",
		Prompt:           "botanical summer",
		SheinStoreID:     9001,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func newStudioBatchItemsForTest(batchID string, now time.Time) []StudioBatchItemRecord {
	return []StudioBatchItemRecord{{
		ID:               "item-1",
		BatchID:          batchID,
		TargetGroupKey:   "size:1200x1200",
		TargetGroupLabel: "1200 x 1200",
		Status:           StudioBatchItemStatusPending,
		SelectionCount:   3,
		CreatedAt:        now,
		UpdatedAt:        now,
	}}
}

func newStudioBatchAttemptsForTest(itemID string, now time.Time) []StudioGenerationAttemptRecord {
	return []StudioGenerationAttemptRecord{{
		ID:        "attempt-1",
		ItemID:    itemID,
		AttemptNo: 1,
		Status:    StudioGenerationAttemptStatusQueued,
		CreatedAt: now,
		UpdatedAt: now,
	}}
}

func newStudioBatchDesignsForTest(batchID string, itemID string, now time.Time) []StudioMaterializedDesignRecord {
	return []StudioMaterializedDesignRecord{{
		ID:              "design-1",
		BatchID:         batchID,
		ItemID:          itemID,
		SourceAttemptID: "attempt-1",
		TargetGroupKey:  "size:1200x1200",
		ImageURL:        "https://cdn.example.com/design-1.png",
		ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
		CreatedAt:       now,
		UpdatedAt:       now,
	}}
}
