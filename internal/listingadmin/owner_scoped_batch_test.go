package listingadmin

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestBatchCreateImportTasksRejectsOwnerlessBatchBeforeInsert(t *testing.T) {
	db := openOwnerScopedBatchSQLite(t)
	if err := db.AutoMigrate(&listingProductImportTask{}); err != nil {
		t.Fatalf("migrate import task: %v", err)
	}
	if err := AutoMigrateImportTaskRepository(db); err != nil {
		t.Fatalf("migrate import task repository: %v", err)
	}
	storeID := int64(986)
	_, err := NewGormImportTaskRepository(db).BatchCreateImportTasks(context.Background(), []ImportTask{
		{TenantID: 246, StoreID: &storeID, Platform: "amazon", TargetPlatform: "shein", Region: "US", ProductID: "A"},
		{TenantID: 246, StoreID: &storeID, Platform: "amazon", TargetPlatform: "shein", Region: "US", ProductID: "B"},
	})
	if !errors.Is(err, ErrOwnerUserIDRequired) {
		t.Fatalf("BatchCreateImportTasks() error = %v, want ErrOwnerUserIDRequired", err)
	}
	var count int64
	if err := db.Table("listing_product_import_task").Count(&count).Error; err != nil {
		t.Fatalf("count import tasks: %v", err)
	}
	if count != 0 {
		t.Fatalf("import task rows = %d, want 0", count)
	}
}

func TestUpsertProductDataBatchRejectsOwnerlessItemWithoutPartialWrite(t *testing.T) {
	db := openOwnerScopedBatchSQLite(t)
	if err := db.AutoMigrate(&listingProductData{}); err != nil {
		t.Fatalf("migrate product data: %v", err)
	}
	_, err := NewGormProductDataRepository(db).UpsertProductDataBatch(context.Background(), []ProductData{
		{TenantID: 246, Platform: "amazon", PlatformProductID: "A", ProductID: "A"},
		{TenantID: 246, Platform: "amazon", PlatformProductID: "B", ProductID: "B"},
	})
	if !errors.Is(err, ErrOwnerUserIDRequired) {
		t.Fatalf("UpsertProductDataBatch() error = %v, want ErrOwnerUserIDRequired", err)
	}
	var count int64
	if err := db.Table("listing_product_data").Count(&count).Error; err != nil {
		t.Fatalf("count product data: %v", err)
	}
	if count != 0 {
		t.Fatalf("product data rows = %d, want 0", count)
	}
}

func TestProductImportMappingRequestIdentityCannotBeOverriddenByPayload(t *testing.T) {
	db := openOwnerScopedBatchSQLite(t)
	if err := db.AutoMigrate(&listingProductImportMapping{}); err != nil {
		t.Fatalf("migrate product import mapping: %v", err)
	}
	mapping, err := NewGormProductImportMappingRepository(db).CreateProductImportMapping(
		WithOwnerUserID(context.Background(), "verified-sub"),
		&ProductImportMapping{
			TenantID:     246,
			OwnerUserID:  "payload-subject",
			ImportTaskID: 1,
			StoreID:      986,
			Platform:     "amazon",
			Region:       "US",
			ProductID:    "A",
		},
	)
	if err != nil {
		t.Fatalf("CreateProductImportMapping() error = %v", err)
	}
	if mapping.OwnerUserID != "verified-sub" {
		t.Fatalf("persisted owner = %q, want verified-sub", mapping.OwnerUserID)
	}
}

func openOwnerScopedBatchSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}
