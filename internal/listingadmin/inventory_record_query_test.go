package listingadmin

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestGormInventoryRecordRepositoryLifecycleHelpers(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingInventoryRecord{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := NewGormInventoryRecordRepository(db)
	stock1 := 10
	stock2 := 15
	first, err := repo.CreateInventoryRecord(context.Background(), &InventoryRecord{
		Platform:    "shein",
		ProductID:   "P-1",
		Region:      "us",
		Stock:       &stock1,
		IsAvailable: true,
		SyncSource:  "initial",
	})
	if err != nil {
		t.Fatalf("CreateInventoryRecord(first) error = %v", err)
	}
	if first == nil || first.ID <= 0 || first.CreateTime == nil {
		t.Fatalf("CreateInventoryRecord(first) = %+v", first)
	}

	time.Sleep(5 * time.Millisecond)
	if _, err := repo.CreateInventoryRecord(context.Background(), &InventoryRecord{
		Platform:    "shein",
		ProductID:   "P-1",
		Region:      "us",
		Stock:       &stock2,
		IsAvailable: true,
		SyncSource:  "latest",
	}); err != nil {
		t.Fatalf("CreateInventoryRecord(second) error = %v", err)
	}

	record, err := repo.GetLatestInventoryRecord(context.Background(), "shein", "P-1", "us")
	if err != nil {
		t.Fatalf("GetLatestInventoryRecord() error = %v", err)
	}
	if record == nil || record.Stock == nil || *record.Stock != stock2 {
		t.Fatalf("GetLatestInventoryRecord() = %+v, want stock %d", record, stock2)
	}
}
