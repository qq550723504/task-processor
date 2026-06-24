package store

import (
	"context"
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
	ctx := context.Background()
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
