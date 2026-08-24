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

func TestUpsertProductDataBatchDerivesOwnerFromStoreInWriteTransaction(t *testing.T) {
	db := openOwnerScopedBatchSQLite(t)
	if err := db.AutoMigrate(&listingStore{}, &listingProductData{}); err != nil {
		t.Fatalf("migrate owner and product data tables: %v", err)
	}
	if err := db.Create(&listingStore{ID: 986, TenantID: 246, OwnerUserID: "store-owner"}).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}
	storeID := int64(986)

	count, err := NewGormProductDataRepository(db).UpsertProductDataBatch(context.Background(), []ProductData{{
		TenantID:          246,
		StoreID:           &storeID,
		Platform:          "shein",
		PlatformProductID: "SP-1",
		ProductID:         "P-1",
	}})
	if err != nil || count != 1 {
		t.Fatalf("UpsertProductDataBatch() = count:%d error:%v, want one persisted row", count, err)
	}
	var owner string
	if err := db.Table("listing_product_data").Pluck("owner_user_id", &owner).Error; err != nil {
		t.Fatalf("load product owner: %v", err)
	}
	if owner != "store-owner" {
		t.Fatalf("persisted owner = %q, want store-owner", owner)
	}
}

func TestProductImportMappingForStoreDerivesOwnerInWriteTransaction(t *testing.T) {
	db := openOwnerScopedBatchSQLite(t)
	if err := db.AutoMigrate(&listingStore{}, &listingProductImportMapping{}); err != nil {
		t.Fatalf("migrate owner and mapping tables: %v", err)
	}
	if err := db.Create(&listingStore{ID: 986, TenantID: 246, OwnerUserID: "store-owner"}).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}

	created, err := NewGormProductImportMappingRepository(db).CreateProductImportMappingForStore(context.Background(), &ProductImportMapping{
		TenantID:     246,
		ImportTaskID: 1,
		StoreID:      986,
		Platform:     "shein",
		Region:       "US",
		ProductID:    "P-1",
		OwnerUserID:  "stale-owner",
	})
	if err != nil || created == nil {
		t.Fatalf("CreateProductImportMappingForStore() = %+v, error:%v", created, err)
	}
	if created.OwnerUserID != "store-owner" {
		t.Fatalf("created owner = %q, want store-owner", created.OwnerUserID)
	}

	created.Remark = "updated"
	created.OwnerUserID = "another-stale-owner"
	updated, err := NewGormProductImportMappingRepository(db).UpdateProductImportMappingForStore(context.Background(), created)
	if err != nil || updated == nil {
		t.Fatalf("UpdateProductImportMappingForStore() = %+v, error:%v", updated, err)
	}
	if updated.OwnerUserID != "store-owner" {
		t.Fatalf("updated owner = %q, want store-owner", updated.OwnerUserID)
	}
}

func TestProductImportMappingForStorePreservesStoreLookupErrors(t *testing.T) {
	db := openOwnerScopedBatchSQLite(t)
	if err := db.AutoMigrate(&listingStore{}, &listingProductImportMapping{}); err != nil {
		t.Fatalf("migrate owner and mapping tables: %v", err)
	}
	repo := NewGormProductImportMappingRepository(db)
	mapping := &ProductImportMapping{
		TenantID:     246,
		ImportTaskID: 1,
		StoreID:      986,
		Platform:     "shein",
		Region:       "US",
		ProductID:    "P-1",
	}
	if _, err := repo.CreateProductImportMappingForStore(context.Background(), mapping); !errors.Is(err, ErrStoreNotFound) {
		t.Fatalf("missing store create error = %v, want ErrStoreNotFound", err)
	}

	if err := db.Create(&listingStore{ID: 986, TenantID: 246}).Error; err != nil {
		t.Fatalf("seed ownerless store: %v", err)
	}
	if _, err := repo.CreateProductImportMappingForStore(context.Background(), mapping); !errors.Is(err, ErrProductImportMappingOwnerRequired) {
		t.Fatalf("ownerless store create error = %v, want ErrProductImportMappingOwnerRequired", err)
	}

	if err := db.Model(&listingStore{}).Where("id = ?", 986).Update("owner_user_id", "store-owner").Error; err != nil {
		t.Fatalf("set store owner: %v", err)
	}
	created, err := repo.CreateProductImportMappingForStore(context.Background(), mapping)
	if err != nil {
		t.Fatalf("create mapping for update: %v", err)
	}
	if err := db.Model(&listingStore{}).Where("id = ?", 986).Update("deleted", 1).Error; err != nil {
		t.Fatalf("delete store: %v", err)
	}
	if _, err := repo.UpdateProductImportMappingForStore(context.Background(), created); !errors.Is(err, ErrStoreNotFound) {
		t.Fatalf("missing store update error = %v, want ErrStoreNotFound", err)
	}
}

func TestProductImportMappingForStorePreservesDatabaseErrors(t *testing.T) {
	db := openOwnerScopedBatchSQLite(t)
	repo := NewGormProductImportMappingRepository(db)
	_, err := repo.CreateProductImportMappingForStore(context.Background(), &ProductImportMapping{
		TenantID:     246,
		ImportTaskID: 1,
		StoreID:      986,
		Platform:     "shein",
		Region:       "US",
		ProductID:    "P-1",
	})
	if err == nil {
		t.Fatal("CreateProductImportMappingForStore() error = nil, want database error")
	}
	if errors.Is(err, ErrProductImportMappingOwnerRequired) {
		t.Fatalf("database error = %v, must not become ErrProductImportMappingOwnerRequired", err)
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
