package local

import (
	"testing"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"task-processor/internal/listingadmin"
)

func TestLocalRuntimeStoreAPIUsesResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeStoreServiceTestDB(t)
	if err := db.Table("listing_store").Create(&localListingStore{
		ID: 81, TenantID: 8, OwnerUserID: "store-owner", StoreID: "SHEIN-81", Name: "resource store", Platform: "shein", Region: "us",
	}).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}

	runtime := &LocalRuntime{resources: NewRuntimeResources(db, nil)}
	storeAPI := runtime.GetStoreAPI()
	if storeAPI == nil {
		t.Fatal("GetStoreAPI() returned nil")
	}
	store, err := storeAPI.GetStore(81)
	if err != nil || store == nil || store.ID != 81 || store.StoreID != "SHEIN-81" || store.Name != "resource store" {
		t.Fatalf("GetStore() = %#v, %v; want persisted resource store", store, err)
	}
	updated, err := storeAPI.UpdateStoreId(&listingadmin.StoreIdUpdateReqDTO{ID: 81, StoreID: "SHEIN-81-UPDATED"})
	if err != nil || !updated {
		t.Fatalf("UpdateStoreId() = %t, %v; want persisted update", updated, err)
	}
	store, err = storeAPI.GetStore(81)
	if err != nil || store == nil || store.StoreID != "SHEIN-81-UPDATED" {
		t.Fatalf("GetStore() after update = %#v, %v; want updated store id", store, err)
	}
}

func TestLocalRuntimeStoreAPIUsesResourcesForPauseStateWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeStoreServiceTestDB(t)
	if err := db.Table("listing_store").Create(&localListingStore{
		ID: 82, TenantID: 8, OwnerUserID: "store-owner", StoreID: "SHEIN-82", Name: "resource store", Platform: "shein", Region: "us",
	}).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}
	server := miniredis.RunT(t)
	resources := NewRuntimeResources(db, goredis.NewClient(&goredis.Options{Addr: server.Addr()}))
	t.Cleanup(func() { _ = resources.Close() })

	storeAPI := (&LocalRuntime{resources: resources}).GetStoreAPI()
	updated, err := storeAPI.SetStorePauseStatus(82, true, "auth_expired")
	if err != nil || !updated {
		t.Fatalf("SetStorePauseStatus(pause) = %t, %v; want persisted pause", updated, err)
	}
	detail, err := storeAPI.GetStorePauseStatusDetail(82)
	if err != nil || detail == nil || !detail.Paused || detail.Reason != "auth_expired" || detail.TTLSeconds <= 0 {
		t.Fatalf("GetStorePauseStatusDetail() = %#v, %v; want resource-backed pause detail", detail, err)
	}
	updated, err = storeAPI.SetStorePauseStatus(82, false, "")
	if err != nil || !updated {
		t.Fatalf("SetStorePauseStatus(unpause) = %t, %v; want cleared pause", updated, err)
	}
	paused, err := storeAPI.GetStorePauseStatus(82)
	if err != nil || paused {
		t.Fatalf("GetStorePauseStatus() = %t, %v; want false after clear", paused, err)
	}
}

func TestLocalRuntimeStoreAPIDeletesCookieFromResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeStoreServiceTestDB(t)
	if err := db.Table("listing_store").Create(&localListingStore{
		ID: 83, TenantID: 8, OwnerUserID: "store-owner", StoreID: "SHEIN-83", Name: "resource store", Platform: "shein", Region: "us",
	}).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}
	server := miniredis.RunT(t)
	server.Set("shein:cookie:8:83", "cookie-value")
	resources := NewRuntimeResources(db, goredis.NewClient(&goredis.Options{Addr: server.Addr()}))
	t.Cleanup(func() { _ = resources.Close() })

	storeAPI := (&LocalRuntime{resources: resources}).GetStoreAPI()
	deleted, err := storeAPI.DeleteStoreCookie(83)
	if err != nil || !deleted {
		t.Fatalf("DeleteStoreCookie() = %t, %v; want resource-backed deletion", deleted, err)
	}
	if server.Exists("shein:cookie:8:83") {
		t.Fatal("DeleteStoreCookie() left the cookie key in Redis")
	}
}

func TestLocalDataProviderStoreStateCompatibility(t *testing.T) {
	db := newLocalRuntimeStoreServiceTestDB(t)
	if err := db.Table("listing_store").Create(&localListingStore{
		ID: 84, TenantID: 8, OwnerUserID: "store-owner", StoreID: "SHEIN-84", Name: "compatibility store", Platform: "shein", Region: "us",
	}).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}
	server := miniredis.RunT(t)
	server.Set("shein:cookie:8:84", "cookie-value")
	resources := NewRuntimeResources(db, goredis.NewClient(&goredis.Options{Addr: server.Addr()}))
	provider := NewLocalDataProviderFromResources(resources)
	t.Cleanup(func() { _ = provider.Close() })

	updated, err := provider.SetStorePauseStatus(84, true, "auth_expired")
	if err != nil || !updated {
		t.Fatalf("SetStorePauseStatus() = %t, %v; want compatibility pause update", updated, err)
	}
	paused, err := provider.GetStorePauseStatus(84)
	if err != nil || !paused {
		t.Fatalf("GetStorePauseStatus() = %t, %v; want compatibility pause state", paused, err)
	}
	deleted, err := provider.DeleteStoreCookie(84)
	if err != nil || !deleted {
		t.Fatalf("DeleteStoreCookie() = %t, %v; want compatibility cookie deletion", deleted, err)
	}
}

func TestLocalDataProviderStoreDatabaseCompatibility(t *testing.T) {
	db := newLocalRuntimeStoreServiceTestDB(t)
	if err := db.Table("listing_store").Create(&localListingStore{
		ID: 85, TenantID: 8, OwnerUserID: "store-owner", StoreID: "SHEIN-85", Name: "compatibility store", Platform: "shein", Region: "us",
	}).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}
	provider := NewLocalDataProviderFromResources(NewRuntimeResources(db, nil))
	t.Cleanup(func() { _ = provider.Close() })

	store, err := provider.GetStore(85)
	if err != nil || store == nil || store.ID != 85 || store.StoreID != "SHEIN-85" {
		t.Fatalf("GetStore() = %#v, %v; want persisted compatibility store", store, err)
	}
	page, err := provider.PageStores(&listingadmin.StorePageReqDTO{Platform: "shein", PageNo: 1, PageSize: 20})
	if err != nil || page == nil || page.Total != 1 || len(page.List) != 1 || page.List[0].ID != 85 {
		t.Fatalf("PageStores() = %#v, %v; want the persisted compatibility store", page, err)
	}
	updated, err := provider.UpdateStoreID(85, "SHEIN-85-UPDATED")
	if err != nil || !updated {
		t.Fatalf("UpdateStoreID() = %t, %v; want successful update", updated, err)
	}
	updated, err = provider.UpdateStoreStatus(85, 1, "duplicate store")
	if err != nil || !updated {
		t.Fatalf("UpdateStoreStatus() = %t, %v; want successful update", updated, err)
	}
	store, err = provider.GetStore(85)
	if err != nil || store == nil || store.StoreID != "SHEIN-85-UPDATED" || store.Status != 1 || store.Remark != "duplicate store" {
		t.Fatalf("GetStore() after updates = %#v, %v; want updated compatibility store", store, err)
	}
}
