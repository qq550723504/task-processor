package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

func newSDSRetirementRepoHarness(t *testing.T) (*taskRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&listingkit.Task{},
		&listingkit.SDSRetirementRunRecord{},
		&listingkit.SDSRetirementItemRecord{},
		&listingkit.SheinSyncedProductRecord{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return &taskRepository{db: db}, db
}

func TestSDSRetirementRepositoryCreatesAndLoadsRunWithItems(t *testing.T) {
	repo, _ := newSDSRetirementRepoHarness(t)
	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	run := &listingkit.SDSRetirementRunRecord{
		ID:               "run-1",
		TenantID:         "tenant-a",
		Platform:         "shein",
		StoreID:          177,
		ParentProductID:  238915,
		PrototypeGroupID: 28345,
		VariantID:        238916,
		Status:           listingkit.SDSRetirementRunStatusReady,
		ReasonCode:       listingkit.SDSBaselineReasonCodeProductDetailCheckFailed,
	}
	items := []listingkit.SDSRetirementItemRecord{{
		ID:            "item-1",
		RunID:         "run-1",
		TenantID:      "tenant-a",
		Platform:      "shein",
		StoreID:       177,
		SPUName:       "SPU-1",
		SKCName:       "SKC-1",
		Selected:      true,
		SiteSelection: `[{"site_abbr":"US","store_type":1}]`,
		Status:        listingkit.SDSRetirementItemStatusSelected,
	}}

	if err := repo.CreateSDSRetirementRun(ctx, run, items); err != nil {
		t.Fatalf("create run: %v", err)
	}
	gotRun, gotItems, err := repo.GetSDSRetirementRun(ctx, "run-1")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if gotRun.ID != "run-1" || gotRun.Status != listingkit.SDSRetirementRunStatusReady {
		t.Fatalf("run = %+v", gotRun)
	}
	if len(gotItems) != 1 || gotItems[0].SKCName != "SKC-1" || !gotItems[0].Selected {
		t.Fatalf("items = %+v", gotItems)
	}
}

func TestSDSRetirementRepositoryGetRunHonorsTenantScope(t *testing.T) {
	repo, _ := newSDSRetirementRepoHarness(t)
	ctxA := listingkit.WithTenantID(context.Background(), "tenant-a")
	ctxB := listingkit.WithTenantID(context.Background(), "tenant-b")
	run := &listingkit.SDSRetirementRunRecord{
		ID:       "run-tenant-a",
		TenantID: "tenant-a",
		Platform: "shein",
		StoreID:  177,
		Status:   listingkit.SDSRetirementRunStatusReady,
	}
	if err := repo.CreateSDSRetirementRun(ctxA, run, []listingkit.SDSRetirementItemRecord{{
		ID:       "item-tenant-a",
		TenantID: "tenant-a",
		Platform: "shein",
		StoreID:  177,
		Status:   listingkit.SDSRetirementItemStatusSelected,
	}}); err != nil {
		t.Fatalf("create run: %v", err)
	}

	if _, _, err := repo.GetSDSRetirementRun(ctxB, run.ID); !errors.Is(err, listingkit.ErrTaskNotFound) {
		t.Fatalf("GetSDSRetirementRun() error = %v, want ErrTaskNotFound for foreign tenant", err)
	}
}

func TestSDSRetirementRepositoryGetRunRequiresExplicitTenantScope(t *testing.T) {
	repo, _ := newSDSRetirementRepoHarness(t)
	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	run := &listingkit.SDSRetirementRunRecord{
		ID:       "run-tenant-a",
		TenantID: "tenant-a",
		Platform: "shein",
		StoreID:  177,
		Status:   listingkit.SDSRetirementRunStatusReady,
	}
	if err := repo.CreateSDSRetirementRun(ctx, run, nil); err != nil {
		t.Fatalf("create run: %v", err)
	}

	if _, _, err := repo.GetSDSRetirementRun(context.Background(), run.ID); !errors.Is(err, listingkit.ErrTaskNotFound) {
		t.Fatalf("GetSDSRetirementRun() error = %v, want ErrTaskNotFound without tenant scope", err)
	}
}

func TestSDSRetirementRepositoryMarksSyncedProductOffShelfAfterSuccess(t *testing.T) {
	repo, db := newSDSRetirementRepoHarness(t)
	ctx := context.Background()
	row := listingkit.SheinSyncedProductRecord{
		TenantID:    1,
		StoreID:     177,
		SKCName:     "SKC-1",
		ShelfStatus: "ON_SHELF",
		IsActive:    true,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("create synced product: %v", err)
	}
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	if err := repo.MarkSyncedProductOffShelf(ctx, row.ID, now); err != nil {
		t.Fatalf("mark off shelf: %v", err)
	}
	var got listingkit.SheinSyncedProductRecord
	if err := db.First(&got, row.ID).Error; err != nil {
		t.Fatalf("reload synced product: %v", err)
	}
	if got.ShelfStatus != "OFF_SHELF" || got.IsActive {
		t.Fatalf("synced product = %+v", got)
	}
}

func TestSDSRetirementRepositoryUpdatesItemsAndSavesExecution(t *testing.T) {
	repo, _ := newSDSRetirementRepoHarness(t)
	ctx := context.Background()
	run := &listingkit.SDSRetirementRunRecord{
		ID:       "run-2",
		TenantID: "tenant-a",
		Platform: "shein",
		StoreID:  177,
		Status:   listingkit.SDSRetirementRunStatusReady,
	}
	items := []listingkit.SDSRetirementItemRecord{{
		ID:       "item-2",
		TenantID: "tenant-a",
		Platform: "shein",
		StoreID:  177,
		SKCName:  "SKC-2",
		Status:   listingkit.SDSRetirementItemStatusPending,
	}}
	if err := repo.CreateSDSRetirementRun(ctx, run, items); err != nil {
		t.Fatalf("create run: %v", err)
	}

	if err := repo.UpdateSDSRetirementItems(ctx, run.ID, []listingkit.SDSRetirementItemSelectionUpdate{{
		ItemID:        "item-2",
		Selected:      true,
		SiteSelection: `[{"site_abbr":"US","store_type":1}]`,
	}}); err != nil {
		t.Fatalf("update items: %v", err)
	}

	finishedAt := time.Date(2026, 6, 24, 13, 0, 0, 0, time.UTC)
	run.Status = listingkit.SDSRetirementRunStatusSucceeded
	run.FinishedAt = &finishedAt
	items[0].RunID = run.ID
	items[0].Selected = true
	items[0].SiteSelection = `[{"site_abbr":"US","store_type":1}]`
	items[0].Status = listingkit.SDSRetirementItemStatusSucceeded
	items[0].FinishedAt = &finishedAt

	if err := repo.SaveSDSRetirementExecution(ctx, run, items); err != nil {
		t.Fatalf("save execution: %v", err)
	}

	gotRun, gotItems, err := repo.GetSDSRetirementRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("get run after save: %v", err)
	}
	if gotRun.Status != listingkit.SDSRetirementRunStatusSucceeded || gotRun.FinishedAt == nil || !gotRun.FinishedAt.Equal(finishedAt) {
		t.Fatalf("run after save = %+v", gotRun)
	}
	if len(gotItems) != 1 || !gotItems[0].Selected || gotItems[0].Status != listingkit.SDSRetirementItemStatusSucceeded {
		t.Fatalf("items after save = %+v", gotItems)
	}
}

func TestSDSRetirementRepositoryUpdateItemsReturnsDomainNotFound(t *testing.T) {
	repo, _ := newSDSRetirementRepoHarness(t)
	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	run := &listingkit.SDSRetirementRunRecord{
		ID:       "run-missing-item",
		TenantID: "tenant-a",
		Platform: "shein",
		StoreID:  177,
		Status:   listingkit.SDSRetirementRunStatusReady,
	}
	if err := repo.CreateSDSRetirementRun(ctx, run, nil); err != nil {
		t.Fatalf("create run: %v", err)
	}

	err := repo.UpdateSDSRetirementItems(ctx, run.ID, []listingkit.SDSRetirementItemSelectionUpdate{{
		ItemID:        "missing-item",
		Selected:      true,
		SiteSelection: `[{"site_abbr":"US","store_type":1}]`,
	}})
	if !errors.Is(err, listingkit.ErrTaskNotFound) {
		t.Fatalf("UpdateSDSRetirementItems error = %v, want ErrTaskNotFound", err)
	}
}

func TestSDSRetirementRepositoryUpdateItemsHonorsTenantScope(t *testing.T) {
	repo, _ := newSDSRetirementRepoHarness(t)
	ctxA := listingkit.WithTenantID(context.Background(), "tenant-a")
	ctxB := listingkit.WithTenantID(context.Background(), "tenant-b")
	run := &listingkit.SDSRetirementRunRecord{
		ID:       "run-scope-update",
		TenantID: "tenant-a",
		Platform: "shein",
		StoreID:  177,
		Status:   listingkit.SDSRetirementRunStatusReady,
	}
	items := []listingkit.SDSRetirementItemRecord{{
		ID:       "item-scope-update",
		TenantID: "tenant-a",
		Platform: "shein",
		StoreID:  177,
		Selected: true,
		Status:   listingkit.SDSRetirementItemStatusSelected,
	}}
	if err := repo.CreateSDSRetirementRun(ctxA, run, items); err != nil {
		t.Fatalf("create run: %v", err)
	}

	err := repo.UpdateSDSRetirementItems(ctxB, run.ID, []listingkit.SDSRetirementItemSelectionUpdate{{
		ItemID:        "item-scope-update",
		Selected:      false,
		SiteSelection: `[{"site_abbr":"US","store_type":1}]`,
	}})
	if !errors.Is(err, listingkit.ErrTaskNotFound) {
		t.Fatalf("UpdateSDSRetirementItems() error = %v, want ErrTaskNotFound for foreign tenant", err)
	}

	gotRun, gotItems, err := repo.GetSDSRetirementRun(ctxA, run.ID)
	if err != nil {
		t.Fatalf("GetSDSRetirementRun() error = %v", err)
	}
	if gotRun == nil || len(gotItems) != 1 || !gotItems[0].Selected {
		t.Fatalf("run/items after foreign update = %#v %#v", gotRun, gotItems)
	}
}

func TestSDSRetirementRepositoryUpdateItemsRequiresExplicitTenantScope(t *testing.T) {
	repo, _ := newSDSRetirementRepoHarness(t)
	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	run := &listingkit.SDSRetirementRunRecord{
		ID:       "run-scope-update",
		TenantID: "tenant-a",
		Platform: "shein",
		StoreID:  177,
		Status:   listingkit.SDSRetirementRunStatusReady,
	}
	items := []listingkit.SDSRetirementItemRecord{{
		ID:       "item-scope-update",
		TenantID: "tenant-a",
		Platform: "shein",
		StoreID:  177,
		Selected: true,
		Status:   listingkit.SDSRetirementItemStatusSelected,
	}}
	if err := repo.CreateSDSRetirementRun(ctx, run, items); err != nil {
		t.Fatalf("create run: %v", err)
	}

	err := repo.UpdateSDSRetirementItems(context.Background(), run.ID, []listingkit.SDSRetirementItemSelectionUpdate{{
		ItemID:        "item-scope-update",
		Selected:      false,
		SiteSelection: `[{"site_abbr":"US","store_type":1}]`,
	}})
	if !errors.Is(err, listingkit.ErrTaskNotFound) {
		t.Fatalf("UpdateSDSRetirementItems() error = %v, want ErrTaskNotFound without tenant scope", err)
	}
}

func TestSDSRetirementRepositorySaveExecutionRejectsItemFromAnotherRun(t *testing.T) {
	repo, _ := newSDSRetirementRepoHarness(t)
	ctx := context.Background()

	runA := &listingkit.SDSRetirementRunRecord{
		ID:       "run-a",
		TenantID: "tenant-a",
		Platform: "shein",
		StoreID:  177,
		Status:   listingkit.SDSRetirementRunStatusReady,
	}
	itemsA := []listingkit.SDSRetirementItemRecord{{
		ID:       "item-a",
		RunID:    "run-a",
		TenantID: "tenant-a",
		Platform: "shein",
		StoreID:  177,
		SKCName:  "SKC-A",
		Status:   listingkit.SDSRetirementItemStatusPending,
	}}
	if err := repo.CreateSDSRetirementRun(ctx, runA, itemsA); err != nil {
		t.Fatalf("create run A: %v", err)
	}

	runB := &listingkit.SDSRetirementRunRecord{
		ID:       "run-b",
		TenantID: "tenant-a",
		Platform: "shein",
		StoreID:  177,
		Status:   listingkit.SDSRetirementRunStatusReady,
	}
	itemsB := []listingkit.SDSRetirementItemRecord{{
		ID:       "item-b",
		RunID:    "run-b",
		TenantID: "tenant-a",
		Platform: "shein",
		StoreID:  177,
		SKCName:  "SKC-B",
		Status:   listingkit.SDSRetirementItemStatusPending,
	}}
	if err := repo.CreateSDSRetirementRun(ctx, runB, itemsB); err != nil {
		t.Fatalf("create run B: %v", err)
	}

	finishedAt := time.Date(2026, 6, 24, 14, 0, 0, 0, time.UTC)
	runA.Status = listingkit.SDSRetirementRunStatusSucceeded
	runA.FinishedAt = &finishedAt
	itemsB[0].Status = listingkit.SDSRetirementItemStatusSucceeded
	itemsB[0].FinishedAt = &finishedAt

	err := repo.SaveSDSRetirementExecution(ctx, runA, itemsB)
	if !errors.Is(err, listingkit.ErrTaskNotFound) {
		t.Fatalf("SaveSDSRetirementExecution error = %v, want ErrTaskNotFound for foreign run item", err)
	}

	_, gotItemsB, err := repo.GetSDSRetirementRun(ctx, runB.ID)
	if err != nil {
		t.Fatalf("get run B: %v", err)
	}
	if len(gotItemsB) != 1 {
		t.Fatalf("run B items = %+v", gotItemsB)
	}
	if gotItemsB[0].Status != listingkit.SDSRetirementItemStatusPending || gotItemsB[0].FinishedAt != nil {
		t.Fatalf("run B item mutated = %+v", gotItemsB[0])
	}
}
