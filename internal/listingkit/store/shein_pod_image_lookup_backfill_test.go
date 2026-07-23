package store_test

import (
	"context"
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
