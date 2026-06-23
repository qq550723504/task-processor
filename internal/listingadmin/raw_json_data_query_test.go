package listingadmin

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func rawJSONDataPtrTime(value time.Time) *time.Time {
	v := value
	return &v
}

func TestGormRawJSONDataRepositoryIgnoresDeletedRowsAndUpserts(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingRawJSONData{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now().UTC()
	rows := []listingRawJSONData{
		{
			Platform:    "amazon",
			ProductID:   "B0TEST123",
			Region:      "us",
			RawJSONData: `{"source":"active"}`,
			CreateTime:  rawJSONDataPtrTime(now.Add(-2 * time.Hour)),
			UpdateTime:  rawJSONDataPtrTime(now.Add(-2 * time.Hour)),
			Creator:     "tester",
			Updater:     "tester",
			Deleted:     0,
		},
		{
			Platform:    "amazon",
			ProductID:   "B0TEST123",
			Region:      "us",
			RawJSONData: `{"source":"deleted"}`,
			CreateTime:  rawJSONDataPtrTime(now.Add(-1 * time.Hour)),
			UpdateTime:  rawJSONDataPtrTime(now.Add(-1 * time.Hour)),
			Creator:     "tester",
			Updater:     "tester",
			Deleted:     1,
		},
	}
	for _, row := range rows {
		if err := db.Table("listing_raw_json_data").Create(&row).Error; err != nil {
			t.Fatalf("seed raw json data: %v", err)
		}
	}

	repo := NewGormRawJSONDataRepository(db)
	got, err := repo.GetLatestRawJSONData(context.Background(), "amazon", "B0TEST123", "us")
	if err != nil {
		t.Fatalf("GetLatestRawJSONData() error = %v", err)
	}
	if got == nil || got.RawJSONData != `{"source":"active"}` {
		t.Fatalf("GetLatestRawJSONData() = %+v", got)
	}

	updated, err := repo.UpsertRawJSONData(context.Background(), &RawJSONData{
		TenantID:     1,
		StoreID:      2,
		ImportTaskID: 3,
		CategoryID:   4,
		Platform:     "amazon",
		ProductID:    "B0TEST123",
		Region:       "us",
		RawJSONData:  `{"source":"updated"}`,
		Creator:      "writer",
	})
	if err != nil {
		t.Fatalf("UpsertRawJSONData(update) error = %v", err)
	}
	if updated == nil || updated.ID != got.ID || updated.RawJSONData != `{"source":"updated"}` {
		t.Fatalf("UpsertRawJSONData(update) = %+v", updated)
	}
	if updated.ImportTaskID != 3 || updated.StoreID != 2 || updated.CategoryID != 4 {
		t.Fatalf("UpsertRawJSONData(update) metadata = %+v", updated)
	}
}

func TestGormRawJSONDataRepositoryCreatesNewRow(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingRawJSONData{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := NewGormRawJSONDataRepository(db)
	created, err := repo.UpsertRawJSONData(context.Background(), &RawJSONData{
		TenantID:     10,
		StoreID:      20,
		ImportTaskID: 30,
		Platform:     "amazon",
		ProductID:    "B0SMALLINT",
		Region:       "us",
		CategoryID:   40,
		RawJSONData:  `{"asin":"B0SMALLINT"}`,
		Creator:      "tester",
	})
	if err != nil {
		t.Fatalf("UpsertRawJSONData(create) error = %v", err)
	}
	if created == nil || created.ID <= 0 || created.RawJSONData == "" {
		t.Fatalf("UpsertRawJSONData(create) = %+v", created)
	}
	if created.ImportTaskID != 30 || created.StoreID != 20 || created.CategoryID != 40 {
		t.Fatalf("UpsertRawJSONData(create) metadata = %+v", created)
	}
}

func TestGormRawJSONDataRepositorySupportsSchemaWithoutTenantID(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE listing_raw_json_data (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		store_id INTEGER,
		import_task_id INTEGER,
		platform TEXT NOT NULL,
		product_id TEXT NOT NULL,
		region TEXT NOT NULL,
		category_id INTEGER,
		raw_json_data TEXT,
		status INTEGER NOT NULL DEFAULT 0,
		creator TEXT,
		updater TEXT,
		create_time DATETIME,
		update_time DATETIME,
		deleted INTEGER NOT NULL DEFAULT 0
	)`).Error; err != nil {
		t.Fatalf("create raw json data table: %v", err)
	}

	repo := NewGormRawJSONDataRepository(db)
	created, err := repo.UpsertRawJSONData(context.Background(), &RawJSONData{
		TenantID:     322,
		StoreID:      976,
		ImportTaskID: 123,
		Platform:     "amazon",
		ProductID:    "B0NOtenant",
		Region:       "us",
		CategoryID:   456,
		RawJSONData:  `{"source":"created"}`,
		Creator:      "tester",
	})
	if err != nil {
		t.Fatalf("UpsertRawJSONData(create without tenant_id column) error = %v", err)
	}
	if created == nil || created.ID <= 0 {
		t.Fatalf("UpsertRawJSONData(create without tenant_id column) = %+v", created)
	}

	updated, err := repo.UpsertRawJSONData(context.Background(), &RawJSONData{
		TenantID:     322,
		StoreID:      976,
		ImportTaskID: 124,
		Platform:     "amazon",
		ProductID:    "B0NOtenant",
		Region:       "us",
		CategoryID:   457,
		RawJSONData:  `{"source":"updated"}`,
		Creator:      "tester",
	})
	if err != nil {
		t.Fatalf("UpsertRawJSONData(update without tenant_id column) error = %v", err)
	}
	if updated == nil || updated.ID != created.ID || updated.RawJSONData != `{"source":"updated"}` {
		t.Fatalf("UpsertRawJSONData(update without tenant_id column) = %+v", updated)
	}
	if updated.ImportTaskID != 124 || updated.StoreID != 976 || updated.CategoryID != 457 {
		t.Fatalf("UpsertRawJSONData(update without tenant_id column) metadata = %+v", updated)
	}
}
