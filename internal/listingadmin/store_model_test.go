package listingadmin

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestListingStoreToStoreUsesAuditFallbacks(t *testing.T) {
	t.Parallel()

	row := listingStore{
		ID:          1,
		TenantID:    101,
		OwnerUserID: "",
		CreatedBy:   "creator-id",
		Creator:     "creator-name",
		UpdatedBy:   "",
		Updater:     "updater-name",
		Name:        "Demo Store",
		Username:    "demo-user",
		Password:    "secret",
		Platform:    "SHEIN",
		ShopType:    "semi",
		Region:      "US",
	}

	store := row.toStore()
	if store.OwnerUserID != "creator-id" {
		t.Fatalf("ownerUserID = %q, want creator-id fallback", store.OwnerUserID)
	}
	if store.CreatedBy != "creator-id" {
		t.Fatalf("createdBy = %q, want creator-id fallback", store.CreatedBy)
	}
	if store.UpdatedBy != "updater-name" {
		t.Fatalf("updatedBy = %q, want updater-name fallback", store.UpdatedBy)
	}
}

func TestApplyStoreCreateDefaultsFillsRegionDailyLimitTypeAndAuditNames(t *testing.T) {
	t.Parallel()

	row := listingStore{
		OwnerUserID: "owner-1",
		CreatedBy:   "created-1",
		UpdatedBy:   "updated-1",
	}

	applyStoreCreateDefaults(&row)

	if row.DailyLimitType != "SPU" {
		t.Fatalf("dailyLimitType = %q, want SPU", row.DailyLimitType)
	}
	if row.Region != "US" {
		t.Fatalf("region = %q, want US", row.Region)
	}
	if row.Creator != "created-1" {
		t.Fatalf("creator = %q, want created-1", row.Creator)
	}
	if row.Updater != "updated-1" {
		t.Fatalf("updater = %q, want updated-1", row.Updater)
	}
}

func TestListingStoreToStoreIncludesAuthorizedBrandFields(t *testing.T) {
	t.Parallel()

	enabled := true
	row := listingStore{
		ID:                       99,
		TenantID:                 101,
		Name:                     "Demo Store",
		Username:                 "demo-user",
		Password:                 "secret",
		Platform:                 "SHEIN",
		ShopType:                 "semi",
		Region:                   "US",
		EnableBrandAuthorization: &enabled,
		AuthorizedBrandCode:      "2fd1n",
		AuthorizedBrandName:      "Logitech",
	}

	store := row.toStore()
	if store.EnableBrandAuthorization == nil || !*store.EnableBrandAuthorization {
		t.Fatalf("EnableBrandAuthorization = %#v, want true", store.EnableBrandAuthorization)
	}
	if store.AuthorizedBrandCode != "2fd1n" || store.AuthorizedBrandName != "Logitech" {
		t.Fatalf("authorized brand = %q/%q, want 2fd1n/Logitech", store.AuthorizedBrandCode, store.AuthorizedBrandName)
	}
}

func TestApplyStoreAccessScopeHonorsTenantAndOwnerWithoutDeletedFilter(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingStore{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	seedRows := []listingStore{
		{TenantID: 101, OwnerUserID: "user-a", Name: "active", Username: "active", Password: "secret", Platform: "SHEIN", ShopType: "semi", Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", Name: "deleted", Username: "deleted", Password: "secret", Platform: "SHEIN", ShopType: "semi", Deleted: 1},
		{TenantID: 101, OwnerUserID: "user-b", Name: "other-owner", Username: "other-owner", Password: "secret", Platform: "SHEIN", ShopType: "semi", Deleted: 0},
	}
	for _, row := range seedRows {
		if err := db.Table("listing_store").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	t.Cleanup(SetOwnerScopeRequiredForTesting(true))

	var rows []listingStore
	err = applyStoreAccessScope(db.Table("listing_store"), StoreQuery{TenantID: 101, OwnerUserID: "user-a"}).
		Where("deleted = 1").
		Find(&rows).Error
	if err != nil {
		t.Fatalf("query rows: %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "deleted" {
		t.Fatalf("rows = %+v, want deleted row for same tenant/owner only", rows)
	}
}

func TestTakeStoreAccessRowRespectsDeletedState(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingStore{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	row := listingStore{TenantID: 101, OwnerUserID: "user-a", Name: "deleted", Username: "deleted", Password: "secret", Platform: "SHEIN", ShopType: "semi", Deleted: 1}
	if err := db.Table("listing_store").Create(&row).Error; err != nil {
		t.Fatalf("seed row: %v", err)
	}

	t.Cleanup(SetOwnerScopeRequiredForTesting(true))
	ctx := withRequestIdentity(context.TODO(), "user-a", nil)

	var loaded listingStore
	err = takeStoreAccessRow(ctx, db.Table("listing_store"), 101, row.ID, 1, &loaded)
	if err != nil {
		t.Fatalf("takeStoreAccessRow: %v", err)
	}
	if loaded.Name != "deleted" {
		t.Fatalf("loaded = %+v, want deleted row", loaded)
	}
}

func TestFindStoreRowsSupportsDeletedListingScope(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingStore{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, row := range []listingStore{
		{TenantID: 101, OwnerUserID: "user-a", Name: "deleted-a", Username: "deleted-a", Password: "secret", Platform: "SHEIN", ShopType: "semi", Deleted: 1},
		{TenantID: 101, OwnerUserID: "user-b", Name: "deleted-b", Username: "deleted-b", Password: "secret", Platform: "SHEIN", ShopType: "semi", Deleted: 1},
		{TenantID: 101, OwnerUserID: "user-a", Name: "active-a", Username: "active-a", Password: "secret", Platform: "SHEIN", ShopType: "semi", Deleted: 0},
	} {
		if err := db.Table("listing_store").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	t.Cleanup(SetOwnerScopeRequiredForTesting(true))
	ctx := withRequestIdentity(context.TODO(), "user-a", nil)

	deleted := int16(1)
	rows, err := findStoreRows(ctx, db.Table("listing_store"), StoreQuery{TenantID: 101, Deleted: &deleted})
	if err != nil {
		t.Fatalf("findStoreRows: %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "deleted-a" {
		t.Fatalf("rows = %+v, want deleted row for user-a only", rows)
	}
}
