package listingadmin

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestFindCategoryRowsUsesRequestOwnerScope(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingCategory{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, row := range []listingCategory{
		{TenantID: 101, OwnerUserID: "user-a", Name: "keep", Code: "A", ParentID: 0, Level: 1, Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-b", Name: "other-owner", Code: "B", ParentID: 0, Level: 1, Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", Name: "deleted", Code: "C", ParentID: 0, Level: 1, Status: 1, Deleted: 1},
	} {
		if err := db.Table("listing_category").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	t.Cleanup(SetOwnerScopeRequiredForTesting(true))
	ctx := withRequestIdentity(context.TODO(), "user-a", nil)

	rows, err := findCategoryRows(ctx, db.Table("listing_category"), CategoryQuery{TenantID: 101})
	if err != nil {
		t.Fatalf("findCategoryRows: %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "keep" {
		t.Fatalf("rows = %+v, want only active owner-scoped row", rows)
	}
}

func TestFindCategoryRowsAppliesResourceFilters(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingCategory{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	parentID := int64(10)
	level := 2
	status := int16(1)
	for _, row := range []listingCategory{
		{TenantID: 101, OwnerUserID: "user-a", Name: "Apparel child", Code: "APP-1", ParentID: 10, Level: 2, Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", Name: "Apparel child", Code: "APP-2", ParentID: 10, Level: 2, Status: 0, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", Name: "Other", Code: "OTH-1", ParentID: 0, Level: 1, Status: 1, Deleted: 0},
	} {
		if err := db.Table("listing_category").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	t.Cleanup(SetOwnerScopeRequiredForTesting(true))
	ctx := withRequestIdentity(context.TODO(), "user-a", nil)

	rows, err := findCategoryRows(ctx, db.Table("listing_category"), CategoryQuery{
		TenantID: 101,
		Name:     "Apparel",
		Code:     "APP-1",
		ParentID: &parentID,
		Level:    &level,
		Status:   &status,
	})
	if err != nil {
		t.Fatalf("findCategoryRows: %v", err)
	}
	if len(rows) != 1 || rows[0].Code != "APP-1" {
		t.Fatalf("rows = %+v, want only fully matched category", rows)
	}
}
