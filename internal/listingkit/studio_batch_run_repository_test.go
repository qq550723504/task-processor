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

func TestMemStudioBatchRunRepositoryCreateAndListItemsInOrder(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRunRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	run := &StudioBatchRunRecord{
		ID:            "run-1",
		Status:        StudioBatchRunStatusPending,
		Mode:          StudioBatchRunModeGenerate,
		FailurePolicy: StudioBatchRunFailurePolicyContinueOnError,
		TotalBatches:  2,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	items := []StudioBatchRunItemRecord{
		{ID: "run-1:2", RunID: "run-1", BatchID: "batch-2", Position: 2, Status: StudioBatchRunItemStatusPending, CreatedAt: now, UpdatedAt: now},
		{ID: "run-1:1", RunID: "run-1", BatchID: "batch-1", Position: 1, Status: StudioBatchRunItemStatusPending, CreatedAt: now, UpdatedAt: now},
	}

	if err := repo.CreateStudioBatchRun(ctx, run, items); err != nil {
		t.Fatalf("CreateStudioBatchRun() error = %v", err)
	}

	gotItems, err := repo.ListStudioBatchRunItems(ctx, "run-1")
	if err != nil {
		t.Fatalf("ListStudioBatchRunItems() error = %v", err)
	}
	if len(gotItems) != 2 {
		t.Fatalf("len(ListStudioBatchRunItems()) = %d, want 2", len(gotItems))
	}
	if gotItems[0].BatchID != "batch-1" || gotItems[1].BatchID != "batch-2" {
		t.Fatalf("got items = %+v, want ordered batch ids", gotItems)
	}
}

func TestMemStudioBatchRunRepositoryUpdateAndGetRunAndItem(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRunRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	run := &StudioBatchRunRecord{
		ID:            "run-1",
		Status:        StudioBatchRunStatusPending,
		Mode:          StudioBatchRunModeGenerate,
		FailurePolicy: StudioBatchRunFailurePolicyContinueOnError,
		TotalBatches:  1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	item := StudioBatchRunItemRecord{
		ID:        "run-1:1",
		RunID:     "run-1",
		BatchID:   "batch-1",
		Position:  1,
		Status:    StudioBatchRunItemStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.CreateStudioBatchRun(ctx, run, []StudioBatchRunItemRecord{item}); err != nil {
		t.Fatalf("CreateStudioBatchRun() error = %v", err)
	}

	run.Status = StudioBatchRunStatusRunning
	run.CurrentBatchID = "batch-1"
	run.CurrentIndex = 1
	run.UpdatedAt = now.Add(time.Second)
	if err := repo.UpdateStudioBatchRun(ctx, run); err != nil {
		t.Fatalf("UpdateStudioBatchRun() error = %v", err)
	}

	item.Status = StudioBatchRunItemStatusRunning
	item.AsyncJobID = "job-1"
	item.UpdatedAt = now.Add(2 * time.Second)
	if err := repo.UpdateStudioBatchRunItem(ctx, &item); err != nil {
		t.Fatalf("UpdateStudioBatchRunItem() error = %v", err)
	}

	gotRun, err := repo.GetStudioBatchRun(ctx, "run-1")
	if err != nil {
		t.Fatalf("GetStudioBatchRun() error = %v", err)
	}
	if gotRun.Status != StudioBatchRunStatusRunning || gotRun.CurrentBatchID != "batch-1" || gotRun.CurrentIndex != 1 {
		t.Fatalf("got run = %+v, want updated running state", gotRun)
	}

	gotItems, err := repo.ListStudioBatchRunItems(ctx, "run-1")
	if err != nil {
		t.Fatalf("ListStudioBatchRunItems() error = %v", err)
	}
	if len(gotItems) != 1 {
		t.Fatalf("len(ListStudioBatchRunItems()) = %d, want 1", len(gotItems))
	}
	if gotItems[0].Status != StudioBatchRunItemStatusRunning || gotItems[0].AsyncJobID != "job-1" {
		t.Fatalf("got item = %+v, want updated item state", gotItems[0])
	}
}

func TestMemStudioBatchRunRepositoryListUnfinishedStudioBatchRunsScopesAndOrdersRuns(t *testing.T) {
	restore := SetOwnerScopeRequiredForTesting(true)
	defer restore()

	repo := NewMemStudioBatchRunRepository()
	baseCtx := WithTenantID(context.Background(), "tenant-a")
	ctxUserA := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-a"})
	ctxUserB := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-b"})
	now := time.Now().UTC()

	mustCreateStudioBatchRunRecordForTest(t, repo, ctxUserA, &StudioBatchRunRecord{
		ID:            "run-2",
		Status:        StudioBatchRunStatusRunning,
		Mode:          StudioBatchRunModeGenerate,
		FailurePolicy: StudioBatchRunFailurePolicyContinueOnError,
		CreatedAt:     now.Add(2 * time.Minute),
		UpdatedAt:     now.Add(2 * time.Minute),
	})
	mustCreateStudioBatchRunRecordForTest(t, repo, ctxUserA, &StudioBatchRunRecord{
		ID:            "run-1",
		Status:        StudioBatchRunStatusPending,
		Mode:          StudioBatchRunModeGenerate,
		FailurePolicy: StudioBatchRunFailurePolicyContinueOnError,
		CreatedAt:     now.Add(1 * time.Minute),
		UpdatedAt:     now.Add(1 * time.Minute),
	})
	mustCreateStudioBatchRunRecordForTest(t, repo, ctxUserA, &StudioBatchRunRecord{
		ID:            "run-3",
		Status:        StudioBatchRunStatusSucceeded,
		Mode:          StudioBatchRunModeGenerate,
		FailurePolicy: StudioBatchRunFailurePolicyContinueOnError,
		CreatedAt:     now.Add(3 * time.Minute),
		UpdatedAt:     now.Add(3 * time.Minute),
	})
	mustCreateStudioBatchRunRecordForTest(t, repo, ctxUserB, &StudioBatchRunRecord{
		ID:            "run-4",
		Status:        StudioBatchRunStatusPending,
		Mode:          StudioBatchRunModeGenerate,
		FailurePolicy: StudioBatchRunFailurePolicyContinueOnError,
		CreatedAt:     now,
		UpdatedAt:     now,
	})

	runs, err := repo.ListUnfinishedStudioBatchRuns(ctxUserA)
	if err != nil {
		t.Fatalf("ListUnfinishedStudioBatchRuns() error = %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("len(ListUnfinishedStudioBatchRuns()) = %d, want 2", len(runs))
	}
	if runs[0].ID != "run-1" || runs[1].ID != "run-2" {
		t.Fatalf("ListUnfinishedStudioBatchRuns() ids = [%s %s], want [run-1 run-2]", runs[0].ID, runs[1].ID)
	}
}

func TestMemStudioBatchRunRepositoryNormalizesChildItemScopeToParentRun(t *testing.T) {
	restore := SetOwnerScopeRequiredForTesting(true)
	defer restore()

	repo := NewMemStudioBatchRunRepository()
	baseCtx := WithTenantID(context.Background(), "tenant-a")
	ctxUserA := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-a"})
	ctxUserB := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-b"})
	now := time.Now().UTC()

	run := &StudioBatchRunRecord{
		ID:            "run-1",
		Status:        StudioBatchRunStatusPending,
		Mode:          StudioBatchRunModeGenerate,
		FailurePolicy: StudioBatchRunFailurePolicyContinueOnError,
		TotalBatches:  1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	item := StudioBatchRunItemRecord{
		ID:        "run-1:1",
		RunID:     "run-1",
		TenantID:  "tenant-b",
		UserID:    "user-b",
		BatchID:   "batch-1",
		Position:  1,
		Status:    StudioBatchRunItemStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := repo.CreateStudioBatchRun(ctxUserA, run, []StudioBatchRunItemRecord{item}); err != nil {
		t.Fatalf("CreateStudioBatchRun() error = %v", err)
	}

	gotItems, err := repo.ListStudioBatchRunItems(ctxUserA, "run-1")
	if err != nil {
		t.Fatalf("ListStudioBatchRunItems() error = %v", err)
	}
	if len(gotItems) != 1 {
		t.Fatalf("len(ListStudioBatchRunItems()) = %d, want 1", len(gotItems))
	}
	if gotItems[0].TenantID != "tenant-a" || gotItems[0].UserID != "user-a" {
		t.Fatalf("got item scope = (%q, %q), want parent scope", gotItems[0].TenantID, gotItems[0].UserID)
	}
	if _, err := repo.ListStudioBatchRunItems(ctxUserB, "run-1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-user ListStudioBatchRunItems() error = %v, want record not found", err)
	}
}

func TestGormStudioBatchRunRepositoryScopesByTenant(t *testing.T) {
	t.Parallel()

	db := openStudioBatchRunSQLiteForTest(t)
	if err := AutoMigrateStudioBatchRunRepository(db); err != nil {
		t.Fatalf("AutoMigrateStudioBatchRunRepository() error = %v", err)
	}

	repo := NewGormStudioBatchRunRepository(db)
	ctxA := WithTenantID(context.Background(), "tenant-a")
	ctxB := WithTenantID(context.Background(), "tenant-b")
	now := time.Now().UTC()

	run := &StudioBatchRunRecord{
		ID:            "run-1",
		Status:        StudioBatchRunStatusPending,
		Mode:          StudioBatchRunModeGenerate,
		FailurePolicy: StudioBatchRunFailurePolicyContinueOnError,
		TotalBatches:  1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	item := StudioBatchRunItemRecord{
		ID:        "run-1:1",
		RunID:     "run-1",
		BatchID:   "batch-1",
		Position:  1,
		Status:    StudioBatchRunItemStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.CreateStudioBatchRun(ctxA, run, []StudioBatchRunItemRecord{item}); err != nil {
		t.Fatalf("CreateStudioBatchRun() error = %v", err)
	}

	if _, err := repo.GetStudioBatchRun(ctxB, "run-1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetStudioBatchRun() error = %v, want record not found", err)
	}
	if _, err := repo.ListStudioBatchRunItems(ctxB, "run-1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("ListStudioBatchRunItems() error = %v, want record not found", err)
	}
}

func TestGormStudioBatchRunRepositoryScopesByUserWhenOwnerScopeEnabled(t *testing.T) {
	restore := SetOwnerScopeRequiredForTesting(true)
	defer restore()

	db := openStudioBatchRunSQLiteForTest(t)
	if err := AutoMigrateStudioBatchRunRepository(db); err != nil {
		t.Fatalf("AutoMigrateStudioBatchRunRepository() error = %v", err)
	}

	repo := NewGormStudioBatchRunRepository(db)
	baseCtx := WithTenantID(context.Background(), "tenant-a")
	ctxUserA := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-a"})
	ctxUserB := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-b"})
	now := time.Now().UTC()

	run := &StudioBatchRunRecord{
		ID:            "run-1",
		Status:        StudioBatchRunStatusPending,
		Mode:          StudioBatchRunModeGenerate,
		FailurePolicy: StudioBatchRunFailurePolicyContinueOnError,
		TotalBatches:  1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	item := StudioBatchRunItemRecord{
		ID:        "run-1:1",
		RunID:     "run-1",
		BatchID:   "batch-1",
		Position:  1,
		Status:    StudioBatchRunItemStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.CreateStudioBatchRun(ctxUserA, run, []StudioBatchRunItemRecord{item}); err != nil {
		t.Fatalf("CreateStudioBatchRun() error = %v", err)
	}

	if _, err := repo.GetStudioBatchRun(ctxUserA, "run-1"); err != nil {
		t.Fatalf("same-user GetStudioBatchRun() error = %v", err)
	}
	if _, err := repo.ListStudioBatchRunItems(ctxUserA, "run-1"); err != nil {
		t.Fatalf("same-user ListStudioBatchRunItems() error = %v", err)
	}
	if _, err := repo.GetStudioBatchRun(ctxUserB, "run-1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-user GetStudioBatchRun() error = %v, want record not found", err)
	}
	if _, err := repo.ListStudioBatchRunItems(ctxUserB, "run-1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-user ListStudioBatchRunItems() error = %v, want record not found", err)
	}
}

func TestGormStudioBatchRunRepositoryNormalizesChildItemScopeToParentRun(t *testing.T) {
	restore := SetOwnerScopeRequiredForTesting(true)
	defer restore()

	db := openStudioBatchRunSQLiteForTest(t)
	if err := AutoMigrateStudioBatchRunRepository(db); err != nil {
		t.Fatalf("AutoMigrateStudioBatchRunRepository() error = %v", err)
	}

	repo := NewGormStudioBatchRunRepository(db)
	baseCtx := WithTenantID(context.Background(), "tenant-a")
	ctxUserA := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-a"})
	ctxUserB := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-b"})
	now := time.Now().UTC()

	run := &StudioBatchRunRecord{
		ID:            "run-1",
		Status:        StudioBatchRunStatusPending,
		Mode:          StudioBatchRunModeGenerate,
		FailurePolicy: StudioBatchRunFailurePolicyContinueOnError,
		TotalBatches:  1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	item := StudioBatchRunItemRecord{
		ID:        "run-1:1",
		RunID:     "run-1",
		TenantID:  "tenant-b",
		UserID:    "user-b",
		BatchID:   "batch-1",
		Position:  1,
		Status:    StudioBatchRunItemStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.CreateStudioBatchRun(ctxUserA, run, []StudioBatchRunItemRecord{item}); err != nil {
		t.Fatalf("CreateStudioBatchRun() error = %v", err)
	}

	gotItems, err := repo.ListStudioBatchRunItems(ctxUserA, "run-1")
	if err != nil {
		t.Fatalf("ListStudioBatchRunItems() error = %v", err)
	}
	if len(gotItems) != 1 {
		t.Fatalf("len(ListStudioBatchRunItems()) = %d, want 1", len(gotItems))
	}
	if gotItems[0].TenantID != "tenant-a" || gotItems[0].UserID != "user-a" {
		t.Fatalf("got item scope = (%q, %q), want parent scope", gotItems[0].TenantID, gotItems[0].UserID)
	}
	if _, err := repo.ListStudioBatchRunItems(ctxUserB, "run-1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-user ListStudioBatchRunItems() error = %v, want record not found", err)
	}
}

func TestGormStudioBatchRunRepositoryUpdateMissingOrOutOfScopeReturnsNotFound(t *testing.T) {
	restore := SetOwnerScopeRequiredForTesting(true)
	defer restore()

	db := openStudioBatchRunSQLiteForTest(t)
	if err := AutoMigrateStudioBatchRunRepository(db); err != nil {
		t.Fatalf("AutoMigrateStudioBatchRunRepository() error = %v", err)
	}

	repo := NewGormStudioBatchRunRepository(db)
	baseCtx := WithTenantID(context.Background(), "tenant-a")
	ctxUserA := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-a"})
	ctxUserB := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-b"})
	now := time.Now().UTC()

	run := &StudioBatchRunRecord{
		ID:            "run-1",
		Status:        StudioBatchRunStatusPending,
		Mode:          StudioBatchRunModeGenerate,
		FailurePolicy: StudioBatchRunFailurePolicyContinueOnError,
		TotalBatches:  1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	item := StudioBatchRunItemRecord{
		ID:        "run-1:1",
		RunID:     "run-1",
		BatchID:   "batch-1",
		Position:  1,
		Status:    StudioBatchRunItemStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.CreateStudioBatchRun(ctxUserA, run, []StudioBatchRunItemRecord{item}); err != nil {
		t.Fatalf("CreateStudioBatchRun() error = %v", err)
	}

	missingRun := &StudioBatchRunRecord{ID: "missing-run", Status: StudioBatchRunStatusRunning, Mode: StudioBatchRunModeGenerate, FailurePolicy: StudioBatchRunFailurePolicyContinueOnError, UpdatedAt: now}
	if err := repo.UpdateStudioBatchRun(ctxUserA, missingRun); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("UpdateStudioBatchRun(missing) error = %v, want record not found", err)
	}

	outOfScopeRun := &StudioBatchRunRecord{ID: "run-1", Status: StudioBatchRunStatusRunning, Mode: StudioBatchRunModeGenerate, FailurePolicy: StudioBatchRunFailurePolicyContinueOnError, UpdatedAt: now}
	if err := repo.UpdateStudioBatchRun(ctxUserB, outOfScopeRun); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("UpdateStudioBatchRun(out-of-scope) error = %v, want record not found", err)
	}

	missingItem := &StudioBatchRunItemRecord{ID: "missing-item", RunID: "run-1", BatchID: "batch-x", Position: 9, Status: StudioBatchRunItemStatusRunning, UpdatedAt: now}
	if err := repo.UpdateStudioBatchRunItem(ctxUserA, missingItem); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("UpdateStudioBatchRunItem(missing) error = %v, want record not found", err)
	}

	outOfScopeItem := &StudioBatchRunItemRecord{ID: "run-1:1", RunID: "run-1", BatchID: "batch-1", Position: 1, Status: StudioBatchRunItemStatusRunning, UpdatedAt: now}
	if err := repo.UpdateStudioBatchRunItem(ctxUserB, outOfScopeItem); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("UpdateStudioBatchRunItem(out-of-scope) error = %v, want record not found", err)
	}
}

func TestGormStudioBatchRunRepositoryListUnfinishedStudioBatchRunsScopesAndOrdersRuns(t *testing.T) {
	restore := SetOwnerScopeRequiredForTesting(true)
	defer restore()

	db := openStudioBatchRunSQLiteForTest(t)
	if err := AutoMigrateStudioBatchRunRepository(db); err != nil {
		t.Fatalf("AutoMigrateStudioBatchRunRepository() error = %v", err)
	}

	repo := NewGormStudioBatchRunRepository(db)
	baseCtx := WithTenantID(context.Background(), "tenant-a")
	ctxUserA := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-a"})
	ctxUserB := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-b"})
	now := time.Now().UTC()

	mustCreateStudioBatchRunRecordForTest(t, repo, ctxUserA, &StudioBatchRunRecord{
		ID:            "run-2",
		Status:        StudioBatchRunStatusRunning,
		Mode:          StudioBatchRunModeGenerate,
		FailurePolicy: StudioBatchRunFailurePolicyContinueOnError,
		CreatedAt:     now.Add(2 * time.Minute),
		UpdatedAt:     now.Add(2 * time.Minute),
	})
	mustCreateStudioBatchRunRecordForTest(t, repo, ctxUserA, &StudioBatchRunRecord{
		ID:            "run-1",
		Status:        StudioBatchRunStatusPending,
		Mode:          StudioBatchRunModeGenerate,
		FailurePolicy: StudioBatchRunFailurePolicyContinueOnError,
		CreatedAt:     now.Add(1 * time.Minute),
		UpdatedAt:     now.Add(1 * time.Minute),
	})
	mustCreateStudioBatchRunRecordForTest(t, repo, ctxUserA, &StudioBatchRunRecord{
		ID:            "run-3",
		Status:        StudioBatchRunStatusFailed,
		Mode:          StudioBatchRunModeGenerate,
		FailurePolicy: StudioBatchRunFailurePolicyContinueOnError,
		CreatedAt:     now.Add(3 * time.Minute),
		UpdatedAt:     now.Add(3 * time.Minute),
	})
	mustCreateStudioBatchRunRecordForTest(t, repo, ctxUserB, &StudioBatchRunRecord{
		ID:            "run-4",
		Status:        StudioBatchRunStatusPending,
		Mode:          StudioBatchRunModeGenerate,
		FailurePolicy: StudioBatchRunFailurePolicyContinueOnError,
		CreatedAt:     now,
		UpdatedAt:     now,
	})

	runs, err := repo.ListUnfinishedStudioBatchRuns(ctxUserA)
	if err != nil {
		t.Fatalf("ListUnfinishedStudioBatchRuns() error = %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("len(ListUnfinishedStudioBatchRuns()) = %d, want 2", len(runs))
	}
	if runs[0].ID != "run-1" || runs[1].ID != "run-2" {
		t.Fatalf("ListUnfinishedStudioBatchRuns() ids = [%s %s], want [run-1 run-2]", runs[0].ID, runs[1].ID)
	}
}

func openStudioBatchRunSQLiteForTest(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

func mustCreateStudioBatchRunRecordForTest(t *testing.T, repo StudioBatchRunRepository, ctx context.Context, run *StudioBatchRunRecord) {
	t.Helper()

	if err := repo.CreateStudioBatchRun(ctx, run, nil); err != nil {
		t.Fatalf("CreateStudioBatchRun() error = %v", err)
	}
}
