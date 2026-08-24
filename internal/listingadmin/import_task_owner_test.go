package listingadmin

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestResolveStoreOwnerUserIDReturnsCanonicalOwner(t *testing.T) {
	db := newImportTaskOwnerSQLite(t)
	if err := db.Table("listing_store").Create(&listingStore{
		ID:          986,
		TenantID:    246,
		OwnerUserID: "store-owner",
		Deleted:     0,
	}).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}

	owner, err := ResolveStoreOwnerUserID(context.Background(), db, 246, 986)
	if err != nil || owner != "store-owner" {
		t.Fatalf("ResolveStoreOwnerUserID() = %q, %v; want store-owner", owner, err)
	}
}

func TestResolveStoreOwnerUserIDRejectsStoreWithoutOwner(t *testing.T) {
	db := newImportTaskOwnerSQLite(t)
	if err := db.Table("listing_store").Create(&listingStore{ID: 986, TenantID: 246, Deleted: 0}).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}

	_, err := ResolveStoreOwnerUserID(context.Background(), db, 246, 986)
	if !errors.Is(err, ErrOwnerUserIDRequired) {
		t.Fatalf("ResolveStoreOwnerUserID() error = %v, want ErrOwnerUserIDRequired", err)
	}
}

func TestResolveStoreOwnerUserIDRejectsCrossTenantStore(t *testing.T) {
	db := newImportTaskOwnerSQLite(t)
	if err := db.Table("listing_store").Create(&listingStore{
		ID:          986,
		TenantID:    999,
		OwnerUserID: "store-owner",
		Deleted:     0,
	}).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}

	_, err := ResolveStoreOwnerUserID(context.Background(), db, 246, 986)
	if !errors.Is(err, ErrStoreNotFound) {
		t.Fatalf("ResolveStoreOwnerUserID() error = %v, want ErrStoreNotFound", err)
	}
}

func newImportTaskOwnerSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingStore{}); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	return db
}
