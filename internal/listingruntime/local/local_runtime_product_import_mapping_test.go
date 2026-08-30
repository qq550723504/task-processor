package local

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestLocalRuntimeProductImportMappingReadersUseResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeProductImportMappingTestDB(t)
	rows := []localRuntimeProductImportMappingRow{
		{ID: 1, TenantID: 1, OwnerUserID: "owner", ImportTaskID: 10, StoreID: 1, Platform: "shein", Region: "us", ProductID: "source-1", SKU: "SKU-ONE", PlatformProductID: "PLATFORM-ONLY", Deleted: 0},
		{ID: 2, TenantID: 1, OwnerUserID: "owner", ImportTaskID: 11, StoreID: 2, Platform: "shein", Region: "us", ProductID: "source-2", SKU: "SKU-TWO", PlatformProductID: "PLATFORM-ONLY", Deleted: 0},
		{ID: 3, TenantID: 1, OwnerUserID: "owner", ImportTaskID: 12, StoreID: 30, Platform: "shein", Region: "us", ProductID: "source-3", SKU: "SKU-STORE", PlatformProductID: "PLATFORM-SKU", Deleted: 0},
		{ID: 4, TenantID: 1, OwnerUserID: "owner", ImportTaskID: 40, StoreID: 4, Platform: "shein", Region: "us", ProductID: "source-4", SKU: "SKU-TASK", PlatformProductID: "PLATFORM-TASK", Deleted: 0},
		{ID: 5, TenantID: 1, OwnerUserID: "owner", ImportTaskID: 50, StoreID: 50, Platform: "shein", Region: "us", ProductID: "source-5", SKU: "SKU-PLATFORM-STORE", PlatformProductID: "PLATFORM-STORE", Deleted: 0},
		{ID: 6, TenantID: 1, OwnerUserID: "owner", ImportTaskID: 60, StoreID: 60, Platform: "shein", Region: "us", ProductID: "PUBLISHED", SKU: "SKU-PUBLISHED", PlatformProductID: "PUBLISHED-PLATFORM", Deleted: 0},
	}
	if err := db.Table("listing_product_import_mapping").Create(&rows).Error; err != nil {
		t.Fatalf("seed product import mappings: %v", err)
	}

	runtime := &LocalRuntime{resources: NewRuntimeResources(db, nil)}
	client := runtime.GetProductImportMappingAPI()
	if client == nil {
		t.Fatal("GetProductImportMappingAPI() returned nil")
	}

	platformOnly, err := client.GetProductImportMappingByPlatformProductId(&listingadmin.ProductImportMappingGetReqDTO{PlatformProductId: "PLATFORM-ONLY"})
	if err != nil || platformOnly == nil || platformOnly.ID != 2 {
		t.Fatalf("GetProductImportMappingByPlatformProductId() = %#v, %v; want latest matching mapping ID 2", platformOnly, err)
	}
	sku, err := client.GetProductImportMappingBySku(&listingadmin.ProductImportMappingGetBySkuReqDTO{Sku: "SKU-STORE", StoreId: 30})
	if err != nil || sku == nil || sku.ID != 3 {
		t.Fatalf("GetProductImportMappingBySku() = %#v, %v; want mapping ID 3", sku, err)
	}
	task, err := client.GetProductImportMappingByTaskAndSku(40, "SKU-TASK")
	if err != nil || task == nil || task.ID != 4 {
		t.Fatalf("GetProductImportMappingByTaskAndSku() = %#v, %v; want mapping ID 4", task, err)
	}
	platformStore, err := client.GetProductImportMappingByPlatformProductIdAndStore(&listingadmin.ProductImportMappingGetByPlatformProductIdAndStoreReqDTO{PlatformProductId: "PLATFORM-STORE", StoreId: 50})
	if err != nil || platformStore == nil || platformStore.ID != 5 {
		t.Fatalf("GetProductImportMappingByPlatformProductIdAndStore() = %#v, %v; want mapping ID 5", platformStore, err)
	}
	exists, err := client.CheckProductExists(&listingadmin.ProductImportMappingCheckReqDTO{StoreId: 60, Platform: "shein", Region: "us", ProductId: "PUBLISHED"})
	if err != nil || !exists {
		t.Fatalf("CheckProductExists() = %v, %v; want true, nil", exists, err)
	}
}

func TestLocalRuntimePublishedProductCheckUsesResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeProductImportMappingTestDB(t)
	if err := db.Table("listing_product_import_mapping").Create(&localRuntimeProductImportMappingRow{
		ID: 10, TenantID: 1, OwnerUserID: "owner", ImportTaskID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "PUBLISHED-RESOURCE", PlatformProductID: "SHEIN-LIVE-100", Deleted: 0,
	}).Error; err != nil {
		t.Fatalf("seed published product: %v", err)
	}

	runtime := &LocalRuntime{resources: NewRuntimeResources(db, nil)}
	exists, err := runtime.RuntimePublishedProductExists(context.Background(), 100, "shein", "us", "PUBLISHED-RESOURCE")
	if err != nil || !exists {
		t.Fatalf("RuntimePublishedProductExists() = %v, %v; want true, nil", exists, err)
	}
}

func TestLocalRuntimeFindProductImportMappingUsesResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeProductImportMappingTestDB(t)
	if err := db.Table("listing_product_import_mapping").Create(&localRuntimeProductImportMappingRow{
		ID: 20, TenantID: 2, OwnerUserID: "owner", ImportTaskID: 200, StoreID: 20, Platform: "temu", Region: "uk", ProductID: "FIND-RESOURCE", SKU: "SKU-FIND-RESOURCE", PlatformProductID: "TEMU-FIND-20", Deleted: 0,
	}).Error; err != nil {
		t.Fatalf("seed mapping: %v", err)
	}

	runtime := &LocalRuntime{resources: NewRuntimeResources(db, nil)}
	mapping, err := runtime.FindRuntimeProductImportMappingByTaskAndSKU(context.Background(), 200, "SKU-FIND-RESOURCE")
	if err != nil || mapping == nil || mapping.ID != 20 || mapping.ProductID != "FIND-RESOURCE" {
		t.Fatalf("FindRuntimeProductImportMappingByTaskAndSKU() = %#v, %v; want persisted mapping ID 20", mapping, err)
	}
}

func TestLocalRuntimeProductImportMappingWriteUsesResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeProductImportMappingTestDB(t)
	runtime := &LocalRuntime{resources: NewRuntimeResources(db, nil)}
	owner := "runtime-owner"
	createdID, err := runtime.CreateRuntimeProductImportMapping(context.Background(), &listingruntime.ProductImportMappingUpsert{
		TenantID: 3, OwnerUserID: owner, ImportTaskID: 300, StoreID: 30, Platform: "shein", Region: "us", ProductID: "CREATED-RESOURCE", SKU: stringPtr("SKU-WRITE-RESOURCE"), PlatformProductID: stringPtr("SHEIN-WRITE-30"),
	})
	if err != nil || createdID == 0 {
		t.Fatalf("CreateRuntimeProductImportMapping() = %d, %v; want persisted mapping ID", createdID, err)
	}

	updatedProductID := "UPDATED-RESOURCE"
	if err := runtime.UpdateRuntimeProductImportMapping(context.Background(), &listingruntime.ProductImportMappingUpsert{
		ID: &createdID, TenantID: 3, OwnerUserID: owner, ImportTaskID: 300, StoreID: 30, Platform: "shein", Region: "us", ProductID: updatedProductID, SKU: stringPtr("SKU-WRITE-RESOURCE"), PlatformProductID: stringPtr("SHEIN-WRITE-30"),
	}); err != nil {
		t.Fatalf("UpdateRuntimeProductImportMapping() error: %v", err)
	}

	mapping, err := runtime.FindRuntimeProductImportMappingByTaskAndSKU(context.Background(), 300, "SKU-WRITE-RESOURCE")
	if err != nil || mapping == nil || mapping.ID != createdID || mapping.ProductID != updatedProductID {
		t.Fatalf("updated runtime mapping = %#v, %v; want ID %d with product %q", mapping, err, createdID, updatedProductID)
	}
}

func TestLocalRuntimeProductImportMappingAPIWritesUseResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeProductImportMappingTestDB(t)
	if err := db.Table("listing_store").AutoMigrate(&localListingStore{}); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	if err := db.Table("listing_store").Create(&localListingStore{
		ID: 86, TenantID: 6, OwnerUserID: "store-owner", StoreID: "SHEIN-86", Name: "mapping store", Platform: "shein", Region: "us",
	}).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}

	client := (&LocalRuntime{resources: NewRuntimeResources(db, nil)}).GetProductImportMappingAPI()
	if client == nil {
		t.Fatal("GetProductImportMappingAPI() returned nil")
	}
	costPrice := 12.5
	sku := "SKU-API-WRITE"
	platformProductID := "SHEIN-API-WRITE"
	createdID, err := client.CreateProductImportMapping(&listingadmin.ProductImportMappingCreateReqDTO{
		TenantID: 6, OwnerUserID: "untrusted-owner", ImportTaskId: 860, StoreId: 86, Platform: "shein", Region: "us", ProductId: "API-CREATED", CostPrice: &costPrice, Sku: &sku, PlatformProductId: &platformProductID,
	})
	if err != nil || createdID == 0 {
		t.Fatalf("CreateProductImportMapping() = %d, %v; want persisted resource-backed mapping", createdID, err)
	}

	updatedProductID := "API-UPDATED"
	if err := client.UpdateProductImportMapping(&listingadmin.ProductImportMappingCreateReqDTO{
		ID: &createdID, TenantID: 6, OwnerUserID: "another-untrusted-owner", ImportTaskId: 860, StoreId: 86, Platform: "shein", Region: "us", ProductId: updatedProductID, CostPrice: &costPrice, Sku: &sku, PlatformProductId: &platformProductID,
	}); err != nil {
		t.Fatalf("UpdateProductImportMapping() error: %v", err)
	}

	mapping, err := client.GetProductImportMappingByPlatformProductId(&listingadmin.ProductImportMappingGetReqDTO{PlatformProductId: platformProductID})
	if err != nil || mapping == nil || mapping.ID != createdID || mapping.ProductId != updatedProductID || mapping.OwnerUserID != "store-owner" {
		t.Fatalf("updated API mapping = %#v, %v; want owner-derived persisted mapping", mapping, err)
	}
}

func TestLocalDataProviderProductImportMappingCompatibility(t *testing.T) {
	db := newLocalRuntimeProductImportMappingTestDB(t)
	if err := db.Table("listing_store").AutoMigrate(&localListingStore{}); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	if err := db.Table("listing_store").Create(&localListingStore{
		ID: 87, TenantID: 6, OwnerUserID: "store-owner", StoreID: "SHEIN-87", Name: "mapping store", Platform: "shein", Region: "us",
	}).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}
	provider := NewLocalDataProviderFromResources(NewRuntimeResources(db, nil))
	t.Cleanup(func() { _ = provider.Close() })

	costPrice := 12.5
	sku := "SKU-PROVIDER-87"
	platformProductID := "SHEIN-PROVIDER-87"
	createdID, err := provider.CreateProductImportMapping(&listingadmin.ProductImportMappingCreateReqDTO{
		TenantID: 6, OwnerUserID: "untrusted-owner", ImportTaskId: 870, StoreId: 87, Platform: "shein", Region: "us", ProductId: "PROVIDER-CREATED", CostPrice: &costPrice, Sku: &sku, PlatformProductId: &platformProductID,
	})
	if err != nil || createdID == 0 {
		t.Fatalf("CreateProductImportMapping() = %d, %v; want persisted owner-derived mapping", createdID, err)
	}

	mapping, found, err := provider.GetProductImportMappingByPlatformProductID(platformProductID)
	if err != nil || !found || mapping == nil || mapping.ID != createdID || mapping.OwnerUserID != "store-owner" {
		t.Fatalf("GetProductImportMappingByPlatformProductID() = %#v, %t, %v; want persisted mapping", mapping, found, err)
	}
	mapping, found, err = provider.GetProductImportMappingByTaskAndSKU(870, sku)
	if err != nil || !found || mapping == nil || mapping.ID != createdID {
		t.Fatalf("GetProductImportMappingByTaskAndSKU() = %#v, %t, %v; want persisted mapping", mapping, found, err)
	}
	mapping, found, err = provider.GetProductImportMappingBySKU(sku, 87)
	if err != nil || !found || mapping == nil || mapping.ID != createdID {
		t.Fatalf("GetProductImportMappingBySKU() = %#v, %t, %v; want persisted mapping", mapping, found, err)
	}
	mapping, found, err = provider.GetProductImportMappingByPlatformProductIDAndStore(platformProductID, 87)
	if err != nil || !found || mapping == nil || mapping.ID != createdID {
		t.Fatalf("GetProductImportMappingByPlatformProductIDAndStore() = %#v, %t, %v; want persisted mapping", mapping, found, err)
	}

	updatedProductID := "PROVIDER-UPDATED"
	updated, err := provider.UpdateProductImportMapping(&listingadmin.ProductImportMappingCreateReqDTO{
		ID: &createdID, TenantID: 6, OwnerUserID: "another-untrusted-owner", ImportTaskId: 870, StoreId: 87, Platform: "shein", Region: "us", ProductId: updatedProductID, CostPrice: &costPrice, Sku: &sku, PlatformProductId: &platformProductID,
	})
	if err != nil || !updated {
		t.Fatalf("UpdateProductImportMapping() = %t, %v; want successful update", updated, err)
	}
	updated, err = provider.UpdateProductImportMapping(&listingadmin.ProductImportMappingCreateReqDTO{})
	if err != nil || updated {
		t.Fatalf("UpdateProductImportMapping() invalid request = %t, %v; want false, nil", updated, err)
	}
	exists, available, err := provider.CheckProductExists(&listingadmin.ProductImportMappingCheckReqDTO{StoreId: 87, Platform: "shein", Region: "us", ProductId: updatedProductID})
	if err != nil || !available || !exists {
		t.Fatalf("CheckProductExists() = %t, %t, %v; want true, true, nil", exists, available, err)
	}
}

type localRuntimeProductImportMappingRow struct {
	ID                      int64      `gorm:"column:id;primaryKey"`
	TenantID                int64      `gorm:"column:tenant_id"`
	OwnerUserID             string     `gorm:"column:owner_user_id"`
	ImportTaskID            int64      `gorm:"column:import_task_id"`
	StoreID                 int64      `gorm:"column:store_id"`
	Platform                string     `gorm:"column:platform"`
	Region                  string     `gorm:"column:region"`
	ProductID               string     `gorm:"column:product_id"`
	SKU                     string     `gorm:"column:sku"`
	CostPrice               float64    `gorm:"column:cost_price"`
	PlatformProductID       string     `gorm:"column:platform_product_id"`
	ParentProductID         string     `gorm:"column:parent_product_id"`
	PlatformParentProductID string     `gorm:"column:platform_parent_product_id"`
	FilterRuleID            int64      `gorm:"column:filter_rule_id"`
	FilterRuleRange         string     `gorm:"column:filter_rule_range"`
	ProfitRuleID            int64      `gorm:"column:profit_rule_id"`
	Status                  int16      `gorm:"column:status"`
	SalePriceMultiplier     float64    `gorm:"column:sale_price_multiplier"`
	DiscountPriceMultiplier float64    `gorm:"column:discount_price_multiplier"`
	Remark                  string     `gorm:"column:remark"`
	Creator                 string     `gorm:"column:creator"`
	CreatedBy               string     `gorm:"column:created_by"`
	CreateTime              *time.Time `gorm:"column:create_time"`
	Updater                 string     `gorm:"column:updater"`
	UpdatedBy               string     `gorm:"column:updated_by"`
	UpdateTime              *time.Time `gorm:"column:update_time"`
	Deleted                 int16      `gorm:"column:deleted"`
}

func stringPtr(value string) *string {
	return &value
}

func newLocalRuntimeProductImportMappingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Table("listing_product_import_mapping").AutoMigrate(&localRuntimeProductImportMappingRow{}); err != nil {
		t.Fatalf("migrate product import mapping: %v", err)
	}
	return db
}
