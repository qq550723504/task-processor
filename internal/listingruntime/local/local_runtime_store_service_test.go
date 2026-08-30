package local

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestLocalRuntimeStoreServiceUsesResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeStoreServiceTestDB(t)
	if err := db.Table("listing_store").Create(&localListingStore{
		ID: 80, TenantID: 8, OwnerUserID: "store-owner", StoreID: "SHEIN-80", Name: "resource store", Platform: "shein", Region: "us",
	}).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}

	runtime := &LocalRuntime{resources: NewRuntimeResources(db, nil)}
	service := runtime.GetRuntimeStoreService()
	if service == nil {
		t.Fatal("GetRuntimeStoreService() returned nil")
	}
	store, err := service.GetStore(80)
	if err != nil || store == nil || store.ID != 80 || store.StoreID != "SHEIN-80" || store.Name != "resource store" {
		t.Fatalf("GetStore() = %#v, %v; want persisted resource store", store, err)
	}
}

func TestLocalRuntimeStoreServiceUsesResourcesForPauseStateWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeStoreServiceTestDB(t)
	if err := db.Table("listing_store").Create(&localListingStore{
		ID: 85, TenantID: 8, OwnerUserID: "store-owner", StoreID: "SHEIN-85", Name: "resource store", Platform: "shein", Region: "us",
	}).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}

	redisServer := miniredis.RunT(t)
	redisClient := goredis.NewClient(&goredis.Options{Addr: redisServer.Addr()})
	resources := NewRuntimeResources(db, redisClient)
	t.Cleanup(func() { _ = resources.Close() })

	service := (&LocalRuntime{resources: resources}).GetRuntimeStoreService()
	updated, err := service.SetStorePauseStatus(85, true, "auth_expired")
	if err != nil || !updated {
		t.Fatalf("SetStorePauseStatus(pause) = %t, %v; want resource-backed pause", updated, err)
	}
	detail, err := service.GetStorePauseStatusDetail(85)
	if err != nil || detail == nil || !detail.Paused || detail.Reason != "auth_expired" || detail.TTLSeconds <= 0 {
		t.Fatalf("GetStorePauseStatusDetail() = %#v, %v; want persisted resource-backed pause detail", detail, err)
	}
	updated, err = service.SetStorePauseStatus(85, false, "")
	if err != nil || !updated {
		t.Fatalf("SetStorePauseStatus(unpause) = %t, %v; want cleared resource-backed pause", updated, err)
	}
	paused, err := service.GetStorePauseStatus(85)
	if err != nil || paused {
		t.Fatalf("GetStorePauseStatus() = %t, %v; want false after clear", paused, err)
	}
}

func newLocalRuntimeStoreServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Table("listing_store").AutoMigrate(&localListingStore{}); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	return db
}
