package local

import (
	"context"
	"testing"
)

func TestLocalRuntimeListsAutoPricingStoresFromResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeStoreServiceTestDB(t)
	enabled := true
	disabled := false
	if err := db.Table("listing_store").Create([]localListingStore{
		{ID: 101, TenantID: 1, StoreID: "SHEIN-101", Name: "enabled shein", Platform: "shein", EnableAutoPrice: &enabled},
		{ID: 102, TenantID: 1, StoreID: "SHEIN-102", Name: "disabled shein", Platform: "shein", EnableAutoPrice: &disabled},
		{ID: 103, TenantID: 1, StoreID: "TEMU-103", Name: "enabled temu", Platform: "temu", EnableAutoPrice: &enabled},
	}).Error; err != nil {
		t.Fatalf("seed stores: %v", err)
	}

	runtime := &LocalRuntime{resources: NewRuntimeResources(db, nil)}
	storeIDs, err := runtime.ListRuntimeAutoPricingStoreIDs(context.Background(), "shein")
	if err != nil {
		t.Fatalf("ListRuntimeAutoPricingStoreIDs() error = %v", err)
	}
	if len(storeIDs) != 1 || storeIDs[0] != 101 {
		t.Fatalf("ListRuntimeAutoPricingStoreIDs() = %v; want [101]", storeIDs)
	}
}

func TestLocalRuntimeLoadsAutoPricingStoreConfigFromResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeStoreServiceTestDB(t)
	enabled := true
	rebargain := true
	if err := db.Table("listing_store").Create(&localListingStore{
		ID: 101, TenantID: 1, StoreID: "SHEIN-101", Name: "resource store", Platform: "shein", EnableAutoPrice: &enabled, EnableRebargain: &rebargain,
	}).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}

	runtime := &LocalRuntime{resources: NewRuntimeResources(db, nil)}
	config, err := runtime.GetAutoPricingStoreConfig(context.Background(), 101)
	if err != nil {
		t.Fatalf("GetAutoPricingStoreConfig() error = %v", err)
	}
	if config == nil || config.Name != "resource store" || config.EnableAutoPrice == nil || !*config.EnableAutoPrice || config.EnableRebargain == nil || !*config.EnableRebargain {
		t.Fatalf("GetAutoPricingStoreConfig() = %#v; want persisted auto-pricing configuration", config)
	}
}
