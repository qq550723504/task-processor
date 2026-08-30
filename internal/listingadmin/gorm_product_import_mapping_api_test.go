package listingadmin

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestGormProductImportMappingAPIKeepsExplicitZeroLookupScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingProductImportMapping{}); err != nil {
		t.Fatalf("create product import mapping table: %v", err)
	}
	if err := AutoMigrateProductImportMappingRepository(db); err != nil {
		t.Fatalf("migrate product import mapping: %v", err)
	}
	for _, mapping := range []listingProductImportMapping{
		{ID: 1, StoreID: 0, ImportTaskID: 0, SKU: "ZERO-SCOPE", PlatformProductID: "ZERO-PLATFORM", Deleted: 0},
		{ID: 2, StoreID: 88, ImportTaskID: 99, SKU: "ZERO-SCOPE", PlatformProductID: "ZERO-PLATFORM", Deleted: 0},
	} {
		if err := db.Table("listing_product_import_mapping").Create(&mapping).Error; err != nil {
			t.Fatalf("seed mapping %#v: %v", mapping, err)
		}
	}

	api := NewGormProductImportMappingAPI(NewGormProductImportMappingRepository(db))

	bySKU, err := api.GetProductImportMappingBySku(&ProductImportMappingGetBySkuReqDTO{StoreId: 0, Sku: "ZERO-SCOPE"})
	if err != nil || bySKU == nil || bySKU.ID != 1 {
		t.Fatalf("GetProductImportMappingBySku() = %#v, %v; want zero-store mapping ID 1", bySKU, err)
	}

	byTask, err := api.GetProductImportMappingByTaskAndSku(0, "ZERO-SCOPE")
	if err != nil || byTask == nil || byTask.ID != 1 {
		t.Fatalf("GetProductImportMappingByTaskAndSku() = %#v, %v; want zero-task mapping ID 1", byTask, err)
	}

	byPlatform, err := api.GetProductImportMappingByPlatformProductIdAndStore(&ProductImportMappingGetByPlatformProductIdAndStoreReqDTO{StoreId: 0, PlatformProductId: "ZERO-PLATFORM"})
	if err != nil || byPlatform == nil || byPlatform.ID != 1 {
		t.Fatalf("GetProductImportMappingByPlatformProductIdAndStore() = %#v, %v; want zero-store mapping ID 1", byPlatform, err)
	}
}
