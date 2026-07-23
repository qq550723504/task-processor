package store_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/store"
)

func TestBackfillSheinPODImageLookupIndexesIsIdempotent(t *testing.T) {
	db := migratedLookupDB(t)
	insertBackfillTask(t, db, makeSheinPODLookupTask("task-1", 869, "SUPPLIER", "SKU-1", time.Now()))

	for range 2 {
		processed, err := store.BackfillSheinPODImageLookupIndexes(context.Background(), db, 1)
		if err != nil {
			t.Fatal(err)
		}
		if processed != 1 {
			t.Fatalf("processed = %d, want 1", processed)
		}
	}

	var count int64
	if err := db.Model(&listingkit.SheinPODImageLookupIndex{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("index count = %d, want 1", count)
	}
}

func TestBackfillSheinPODImageLookupIndexesDeletesStaleIndex(t *testing.T) {
	db := migratedLookupDB(t)
	task := makeSheinPODLookupTask("task-stale", 869, "SUPPLIER", "SKU-1", time.Now())
	insertBackfillTask(t, db, task)
	if _, err := store.BackfillSheinPODImageLookupIndexes(context.Background(), db, 1); err != nil {
		t.Fatal(err)
	}

	if err := db.Model(&listingkit.Task{}).
		Where("id = ?", task.ID).
		Update("result", nil).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.BackfillSheinPODImageLookupIndexes(context.Background(), db, 1); err != nil {
		t.Fatal(err)
	}

	var count int64
	if err := db.Model(&listingkit.SheinPODImageLookupIndex{}).
		Where("task_id = ?", task.ID).
		Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("stale index count = %d, want 0", count)
	}
}

func TestBackfillSheinPODImageLookupIndexesRereadsTaskBeforeOverwritingIndex(t *testing.T) {
	db := migratedLookupDB(t)
	repo := store.NewTaskRepository(db)
	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	task := makeSheinPODLookupTask("task-concurrent-update", 869, "SUPPLIER", "SKU-OLD", time.Now())
	insertBackfillTask(t, db, task)

	updatedResult := *task.Result
	updatedResult.Shein.RequestDraft.SKCList[0].SKUList[0].SupplierSKU = "SKU-NEWER"
	var callbackErr error
	triggered := false
	if err := db.Callback().Query().After("gorm:query").Register(
		"test:update_task_after_backfill_batch_read",
		func(tx *gorm.DB) {
			if triggered {
				return
			}
			if _, ok := tx.Statement.Dest.(*[]listingkit.Task); !ok {
				return
			}
			triggered = true
			callbackErr = repo.SaveTaskResult(ctx, task.ID, &updatedResult)
		},
	); err != nil {
		t.Fatalf("register query callback: %v", err)
	}

	if _, err := store.BackfillSheinPODImageLookupIndexes(ctx, db, 1); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if callbackErr != nil {
		t.Fatalf("concurrent task update: %v", callbackErr)
	}
	if !triggered {
		t.Fatal("concurrent task update callback was not triggered")
	}

	var index listingkit.SheinPODImageLookupIndex
	if err := db.First(&index, "task_id = ?", task.ID).Error; err != nil {
		t.Fatalf("load index: %v", err)
	}
	if !strings.EqualFold(index.SellerSKU, "SKU-NEWER") {
		t.Fatalf("indexed seller SKU = %q, want concurrent value", index.SellerSKU)
	}
}

func TestBackfillSheinPODImageLookupIndexesRereadsTaskBeforeDeletingIndex(t *testing.T) {
	db := migratedLookupDB(t)
	repo := store.NewTaskRepository(db)
	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	task := makeSheinPODLookupTask("task-concurrent-create", 869, "SUPPLIER", "SKU-NEW", time.Now())
	result := task.Result
	task.Result = nil
	insertBackfillTask(t, db, task)

	var callbackErr error
	triggered := false
	if err := db.Callback().Query().After("gorm:query").Register(
		"test:add_result_after_backfill_batch_read",
		func(tx *gorm.DB) {
			if triggered {
				return
			}
			if _, ok := tx.Statement.Dest.(*[]listingkit.Task); !ok {
				return
			}
			triggered = true
			callbackErr = repo.SaveTaskResult(ctx, task.ID, result)
		},
	); err != nil {
		t.Fatalf("register query callback: %v", err)
	}

	if _, err := store.BackfillSheinPODImageLookupIndexes(ctx, db, 1); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if callbackErr != nil {
		t.Fatalf("concurrent task update: %v", callbackErr)
	}
	if !triggered {
		t.Fatal("concurrent task update callback was not triggered")
	}

	var count int64
	if err := db.Model(&listingkit.SheinPODImageLookupIndex{}).
		Where("task_id = ?", task.ID).
		Count(&count).Error; err != nil {
		t.Fatalf("count index: %v", err)
	}
	if count != 1 {
		t.Fatalf("index count = %d, want concurrent index preserved", count)
	}
}

func TestAutoMigrateSheinPODImageLookupIndexReplacesLegacyTextIndexes(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&legacySheinPODImageLookupIndex{}); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	if !db.Migrator().HasIndex(&legacySheinPODImageLookupIndex{}, "idx_shein_pod_image_lookup_seller_sku") {
		t.Fatal("legacy text index was not created")
	}

	if err := store.AutoMigrateSheinPODImageLookupIndex(db); err != nil {
		t.Fatalf("migrate lookup index: %v", err)
	}
	if db.Migrator().HasIndex(&listingkit.SheinPODImageLookupIndex{}, "idx_shein_pod_image_lookup_seller_sku") {
		t.Fatal("legacy unbounded normalized text index still exists")
	}
	if !db.Migrator().HasIndex(&listingkit.SheinPODImageLookupIndex{}, "idx_shein_pod_image_lookup_seller_sku_key") {
		t.Fatal("fixed-length seller SKU lookup-key index was not created")
	}
}

type legacySheinPODImageLookupIndex struct {
	TaskID              string `gorm:"primaryKey;type:varchar(36)"`
	StoreID             int64  `gorm:"index:idx_shein_pod_image_lookup_seller_sku,priority:1"`
	NormalizedSellerSKU string `gorm:"type:text;index:idx_shein_pod_image_lookup_seller_sku,priority:2"`
}

func (legacySheinPODImageLookupIndex) TableName() string {
	return "listingkit_shein_pod_image_indexes"
}

func migratedLookupDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}, &listingkit.SheinPODImageLookupIndex{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func insertBackfillTask(t *testing.T, db *gorm.DB, task *listingkit.Task) {
	t.Helper()

	if err := db.Create(task).Error; err != nil {
		t.Fatalf("insert task %q: %v", task.ID, err)
	}
}
