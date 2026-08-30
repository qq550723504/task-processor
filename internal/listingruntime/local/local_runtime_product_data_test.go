package local

import (
	"testing"

	"task-processor/internal/listingadmin"
	"task-processor/internal/pkg/types"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestLocalRuntimeProductDataReadersUseResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeProductDataTestDB(t)
	storeID := int64(50)
	onShelf := 2
	offShelf := 3
	rows := []listingadmin.ProductData{
		{ID: 1, TenantID: 5, StoreID: &storeID, Platform: "shein", Region: "us", ProductID: "LIST-ON-SHELF", PlatformProductID: "PLATFORM-LIST-ON", Title: "red dress", Brand: "Acme", ShelfStatus: &onShelf},
		{ID: 2, TenantID: 5, StoreID: &storeID, Platform: "shein", Region: "us", ProductID: "PAGE-BLUE", PlatformProductID: "PLATFORM-PAGE-BLUE", Title: "blue dress", Brand: "Acme", ShelfStatus: &offShelf},
		{ID: 3, TenantID: 5, StoreID: &storeID, Platform: "shein", Region: "us", ProductID: "OTHER-BRAND", PlatformProductID: "PLATFORM-OTHER", Title: "blue dress", Brand: "Other", ShelfStatus: &offShelf},
	}
	if err := db.Table("listing_product_data").Create(&rows).Error; err != nil {
		t.Fatalf("seed product data: %v", err)
	}

	runtime := &LocalRuntime{resources: NewRuntimeResources(db, nil)}
	client := runtime.GetProductDataClient(storeID)
	if client == nil {
		t.Fatal("GetProductDataClient() returned nil")
	}

	listed, err := client.ListByStore("shein", 5, 0, &onShelf)
	if err != nil || len(listed) != 1 || listed[0].ProductID != "LIST-ON-SHELF" {
		t.Fatalf("ListByStore() = %#v, %v; want the on-shelf default-store product", listed, err)
	}
	page, err := client.PageProductDataByStore(&listingadmin.ProductDataListByStorePageReqDTO{
		Platform: "shein", TenantID: 5, StoreID: storeID, Title: "blue", Brand: "Acme", PageNo: 1, PageSize: 10,
	})
	if err != nil || page == nil || page.Total != 1 || len(page.List) != 1 || page.List[0].ProductID != "PAGE-BLUE" {
		t.Fatalf("PageProductDataByStore() = %#v, %v; want one Acme blue product", page, err)
	}
}

func TestLocalRuntimeProductDataAPIBatchSaveUsesResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeProductDataTestDB(t)
	if err := db.Table("listing_store").AutoMigrate(&localListingStore{}); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	if err := db.Table("listing_store").Create(&localListingStore{
		ID: 51, TenantID: 5, OwnerUserID: "store-owner", StoreID: "SHEIN-51", Name: "product data store", Platform: "shein", Region: "us",
	}).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}

	client := (&LocalRuntime{resources: NewRuntimeResources(db, nil)}).GetProductDataClient(51)
	count, err := client.BatchCreateOrUpdate(&listingadmin.ProductDataBatchSaveReqDTO{
		Platform: "shein", TenantID: 5, Region: "us", StoreID: 51,
		Products: []listingadmin.ProductDataItemDTO{{
			PlatformProductID: "SHEIN-PRODUCT-51", ProductName: "resource product", ProductSku: "SKU-51", ProductPrice: types.FlexibleString("12.5"), ProductStock: types.FlexibleString("8"), Attributes: `{"color":"red"}`,
		}},
	})
	if err != nil || count != 1 {
		t.Fatalf("BatchCreateOrUpdate() = %d, %v; want one resource-backed write", count, err)
	}

	products, err := client.ListByStore("shein", 5, 51, nil)
	if err != nil || len(products) != 1 || products[0].ProductID != "SKU-51" || products[0].Attributes != `{"color":"red"}` {
		t.Fatalf("ListByStore() = %#v, %v; want persisted product data", products, err)
	}
	var owner string
	if err := db.Table("listing_product_data").Where("platform_product_id = ?", "SHEIN-PRODUCT-51").Pluck("owner_user_id", &owner).Error; err != nil {
		t.Fatalf("load derived owner: %v", err)
	}
	if owner != "store-owner" {
		t.Fatalf("persisted owner = %q, want store-derived owner", owner)
	}
}

func TestLocalRuntimeProductDataAPIAttributeBatchUpdateUsesResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeProductDataTestDB(t)
	storeID := int64(52)
	if err := db.Table("listing_product_data").Create(&listingadmin.ProductData{
		TenantID: 5, StoreID: &storeID, Platform: "shein", Region: "us", ProductID: "SKU-52", PlatformProductID: "SHEIN-PRODUCT-52", Attributes: []byte(`{"color":"red"}`),
	}).Error; err != nil {
		t.Fatalf("seed product data: %v", err)
	}

	client := (&LocalRuntime{resources: NewRuntimeResources(db, nil)}).GetProductDataClient(storeID)
	count, err := client.BatchUpdateAttributes(&listingadmin.ProductDataBatchUpdateAttributesReqDTO{
		Platform: "shein", TenantID: 5, StoreID: storeID,
		Products: []listingadmin.ProductAttributesItemDTO{{PlatformProductID: "SHEIN-PRODUCT-52", Attributes: `{"color":"blue"}`}},
	})
	if err != nil || count != 1 {
		t.Fatalf("BatchUpdateAttributes() = %d, %v; want one resource-backed update", count, err)
	}

	products, err := client.ListByStore("shein", 5, storeID, nil)
	if err != nil || len(products) != 1 || products[0].Attributes != `{"color":"blue"}` {
		t.Fatalf("ListByStore() = %#v, %v; want updated attributes", products, err)
	}
}

func TestLocalDataProviderProductDataCompatibility(t *testing.T) {
	db := newLocalRuntimeProductDataTestDB(t)
	if err := db.Table("listing_store").AutoMigrate(&localListingStore{}); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	if err := db.Table("listing_store").Create(&localListingStore{
		ID: 53, TenantID: 5, OwnerUserID: "store-owner", StoreID: "SHEIN-53", Name: "product data store", Platform: "shein", Region: "us",
	}).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}
	storeID := int64(53)
	shelfStatus := 2
	if err := db.Table("listing_product_data").Create(&listingadmin.ProductData{
		ID: 53, TenantID: 5, StoreID: &storeID, Platform: "shein", Region: "us", ProductID: "SKU-53-EXISTING", PlatformProductID: "SHEIN-PRODUCT-53-EXISTING", Title: "blue dress", Brand: "Acme", ShelfStatus: &shelfStatus,
	}).Error; err != nil {
		t.Fatalf("seed product data: %v", err)
	}
	provider := NewLocalDataProviderFromResources(NewRuntimeResources(db, nil))
	t.Cleanup(func() { _ = provider.Close() })

	listed, err := provider.ListProductDataByStore("shein", 5, storeID, &shelfStatus)
	if err != nil || len(listed) != 1 || listed[0].ProductID != "SKU-53-EXISTING" {
		t.Fatalf("ListProductDataByStore() = %#v, %v; want the persisted product", listed, err)
	}
	page, err := provider.PageProductDataByStore(&listingadmin.ProductDataListByStorePageReqDTO{
		Platform: "shein", TenantID: 5, StoreID: storeID, Title: "blue", Brand: "Acme", PageNo: 1, PageSize: 10,
	})
	if err != nil || page == nil || page.Total != 1 || len(page.List) != 1 || page.List[0].ProductID != "SKU-53-EXISTING" {
		t.Fatalf("PageProductDataByStore() = %#v, %v; want the persisted product", page, err)
	}
	count, err := provider.BatchCreateOrUpdateProductData(&listingadmin.ProductDataBatchSaveReqDTO{
		Platform: "shein", TenantID: 5, Region: "us", StoreID: storeID,
		Products: []listingadmin.ProductDataItemDTO{{
			PlatformProductID: "SHEIN-PRODUCT-53", ProductName: "compatibility product", ProductSku: "SKU-53", ProductPrice: types.FlexibleString("12.5"), ProductStock: types.FlexibleString("8"), Attributes: `{"color":"red"}`,
		}},
	})
	if err != nil || count != 1 {
		t.Fatalf("BatchCreateOrUpdateProductData() = %d, %v; want one persisted product", count, err)
	}
	count, err = provider.BatchUpdateProductAttributes(&listingadmin.ProductDataBatchUpdateAttributesReqDTO{
		Platform: "shein", TenantID: 5, StoreID: storeID,
		Products: []listingadmin.ProductAttributesItemDTO{{PlatformProductID: "SHEIN-PRODUCT-53", Attributes: `{"color":"blue"}`}},
	})
	if err != nil || count != 1 {
		t.Fatalf("BatchUpdateProductAttributes() = %d, %v; want one updated product", count, err)
	}
	page, err = provider.PageProductDataByStore(&listingadmin.ProductDataListByStorePageReqDTO{
		Platform: "shein", TenantID: 5, StoreID: storeID, PlatformProductID: "SHEIN-PRODUCT-53", PageNo: 1, PageSize: 10,
	})
	if err != nil || page == nil || page.Total != 1 || len(page.List) != 1 || page.List[0].Attributes != `{"color":"blue"}` {
		t.Fatalf("PageProductDataByStore() after attribute update = %#v, %v; want updated attributes", page, err)
	}
}

func newLocalRuntimeProductDataTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Table("listing_product_data").AutoMigrate(&listingadmin.ProductData{}); err != nil {
		t.Fatalf("migrate product data: %v", err)
	}
	for _, column := range []string{"creator", "updater"} {
		if err := db.Exec("ALTER TABLE listing_product_data ADD COLUMN " + column + " varchar(128)").Error; err != nil {
			t.Fatalf("add product audit column %s: %v", column, err)
		}
	}
	if err := db.Exec("ALTER TABLE listing_product_data ADD COLUMN deleted integer NOT NULL DEFAULT 0").Error; err != nil {
		t.Fatalf("add deleted column: %v", err)
	}
	if err := listingadmin.AutoMigrateProductDataRepository(db); err != nil {
		t.Fatalf("migrate product data repository: %v", err)
	}
	return db
}
