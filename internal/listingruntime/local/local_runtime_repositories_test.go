package local

import (
	"context"
	"testing"

	"task-processor/internal/listingadmin"
)

func TestLocalRuntimeRepositoryGettersUseResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeProductDataTestDB(t)
	storeID := int64(70)
	if err := db.Table("listing_product_data").Create(&listingadmin.ProductData{
		ID: 70, TenantID: 7, StoreID: &storeID, Platform: "shein", ProductID: "RESOURCE-REPOSITORY",
	}).Error; err != nil {
		t.Fatalf("seed product data: %v", err)
	}

	runtime := &LocalRuntime{resources: NewRuntimeResources(db, nil)}
	productRepo := runtime.GetLocalProductDataRepository()
	page, err := productRepo.ListProductData(context.Background(), listingadmin.ProductDataQuery{TenantID: 7, StoreID: &storeID, Page: 1, PageSize: 10})
	if err != nil || page == nil || len(page.Items) != 1 || page.Items[0].ProductID != "RESOURCE-REPOSITORY" {
		t.Fatalf("resource product repository query = %#v, %v; want persisted product", page, err)
	}

	if runtime.GetLocalSheinSyncRepository() == nil ||
		runtime.GetLocalPricingRuleRepository() == nil ||
		runtime.GetLocalProductImportMappingRepository() == nil ||
		runtime.GetLocalStoreRepository() == nil ||
		runtime.GetLocalInventoryRecordRepository() == nil ||
		runtime.GetLocalFilterRuleRepository() == nil ||
		runtime.GetLocalProfitRuleRepository() == nil {
		t.Fatal("resource-only runtime returned a nil repository getter")
	}
}
