package listingadmin

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestFindProductImportMappingRowsUsesRequestOwnerScope(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingProductImportMapping{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, row := range []listingProductImportMapping{
		{TenantID: 101, OwnerUserID: "user-a", ImportTaskID: 1, StoreID: 11, Platform: "SHEIN", Region: "US", ProductID: "P-1", Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-b", ImportTaskID: 2, StoreID: 12, Platform: "SHEIN", Region: "US", ProductID: "P-2", Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", ImportTaskID: 3, StoreID: 13, Platform: "SHEIN", Region: "US", ProductID: "P-3", Status: 1, Deleted: 1},
	} {
		if err := db.Table("listing_product_import_mapping").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	t.Cleanup(SetOwnerScopeRequiredForTesting(true))
	ctx := withRequestIdentity(context.TODO(), "user-a", nil)

	rows, total, page, pageSize, err := findProductImportMappingRows(ctx, db.Table("listing_product_import_mapping"), ProductImportMappingQuery{
		TenantID: 101,
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("findProductImportMappingRows: %v", err)
	}
	if total != 1 || page != 1 || pageSize != 20 {
		t.Fatalf("result meta = total:%d page:%d pageSize:%d, want 1/1/20", total, page, pageSize)
	}
	if len(rows) != 1 || rows[0].ProductID != "P-1" {
		t.Fatalf("rows = %+v, want only active owner-scoped row", rows)
	}
}

func TestFindProductImportMappingRowsAppliesResourceFilters(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingProductImportMapping{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	importTaskID := int64(10)
	storeID := int64(20)
	status := int16(3)
	for _, row := range []listingProductImportMapping{
		{TenantID: 101, OwnerUserID: "user-a", ImportTaskID: 10, StoreID: 20, Platform: "SHEIN", Region: "US", ProductID: "P-1", ParentProductID: "PP-1", SKU: "SKU-1", PlatformProductID: "SP-1", PlatformParentProductID: "SPP-1", Status: 3, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", ImportTaskID: 10, StoreID: 20, Platform: "AMZ", Region: "US", ProductID: "P-1", ParentProductID: "PP-1", SKU: "SKU-1", PlatformProductID: "SP-1", PlatformParentProductID: "SPP-1", Status: 3, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", ImportTaskID: 10, StoreID: 21, Platform: "SHEIN", Region: "US", ProductID: "P-2", ParentProductID: "PP-2", SKU: "SKU-2", PlatformProductID: "SP-2", PlatformParentProductID: "SPP-2", Status: 3, Deleted: 0},
	} {
		if err := db.Table("listing_product_import_mapping").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	t.Cleanup(SetOwnerScopeRequiredForTesting(true))
	ctx := withRequestIdentity(context.TODO(), "user-a", nil)

	rows, total, _, _, err := findProductImportMappingRows(ctx, db.Table("listing_product_import_mapping"), ProductImportMappingQuery{
		TenantID:                101,
		ImportTaskID:            &importTaskID,
		StoreID:                 &storeID,
		Platform:                "SHEIN",
		Region:                  "US",
		ProductID:               "P-1",
		ParentProductID:         "PP-1",
		SKU:                     "SKU-1",
		PlatformProductID:       "SP-1",
		PlatformParentProductID: "SPP-1",
		Status:                  &status,
	})
	if err != nil {
		t.Fatalf("findProductImportMappingRows: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].Platform != "SHEIN" {
		t.Fatalf("rows = %+v total=%d, want only fully matched SHEIN row", rows, total)
	}
}

func TestGormProductImportMappingRepositoryFindLatestAndExistsPublished(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingProductImportMapping{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, row := range []listingProductImportMapping{
		{TenantID: 101, ImportTaskID: 10, StoreID: 20, Platform: "SHEIN", Region: "US", ProductID: "P-1", SKU: "SKU-1", PlatformProductID: "", Status: 1, Deleted: 0},
		{TenantID: 101, ImportTaskID: 10, StoreID: 20, Platform: "SHEIN", Region: "US", ProductID: "P-1", SKU: "SKU-1", PlatformProductID: "SP-NEW", Status: 2, Deleted: 0},
	} {
		if err := db.Table("listing_product_import_mapping").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	repo := NewGormProductImportMappingRepository(db)
	storeID := int64(20)
	importTaskID := int64(10)
	mapping, err := repo.FindLatest(context.Background(), ProductImportMappingQuery{
		StoreID:      &storeID,
		ImportTaskID: &importTaskID,
		SKU:          "SKU-1",
	})
	if err != nil {
		t.Fatalf("FindLatest() error = %v", err)
	}
	if mapping == nil || mapping.PlatformProductID != "SP-NEW" {
		t.Fatalf("FindLatest() = %+v, want newest mapping", mapping)
	}

	exists, err := repo.ExistsPublishedProduct(context.Background(), 20, "SHEIN", "US", "P-1")
	if err != nil {
		t.Fatalf("ExistsPublishedProduct() error = %v", err)
	}
	if !exists {
		t.Fatal("ExistsPublishedProduct() = false, want true")
	}
}
