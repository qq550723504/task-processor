package listingadmin

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestFindPricingRuleRowsUsesRequestOwnerScope(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingPricingRule{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, row := range []listingPricingRule{
		{TenantID: 101, OwnerUserID: "user-a", Name: "keep", RuleCode: "R1", RuleType: "ratio", Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-b", Name: "other-owner", RuleCode: "R2", RuleType: "ratio", Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", Name: "deleted", RuleCode: "R3", RuleType: "ratio", Status: 1, Deleted: 1},
	} {
		if err := db.Table("listing_pricing_rule").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	t.Cleanup(SetOwnerScopeRequiredForTesting(true))
	ctx := withRequestIdentity(context.TODO(), "user-a", nil)

	rows, total, page, pageSize, err := findPricingRuleRows(ctx, db.Table("listing_pricing_rule"), PricingRuleQuery{
		TenantID: 101,
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("findPricingRuleRows: %v", err)
	}
	if total != 1 || page != 1 || pageSize != 20 {
		t.Fatalf("result meta = total:%d page:%d pageSize:%d, want 1/1/20", total, page, pageSize)
	}
	if len(rows) != 1 || rows[0].Name != "keep" {
		t.Fatalf("rows = %+v, want only active owner-scoped row", rows)
	}
}

func TestFindPricingRuleRowsAppliesResourceFilters(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingPricingRule{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	storeID := int64(11)
	categoryID := int64(22)
	status := int16(2)
	for _, row := range []listingPricingRule{
		{TenantID: 101, OwnerUserID: "user-a", Name: "Alpha", RuleCode: "P-1", StoreID: 11, CategoryID: 22, RuleType: "ratio", Status: 2, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", Name: "Alpha", RuleCode: "P-2", StoreID: 11, CategoryID: 22, RuleType: "fixed", Status: 2, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", Name: "Beta", RuleCode: "P-3", StoreID: 12, CategoryID: 22, RuleType: "ratio", Status: 2, Deleted: 0},
	} {
		if err := db.Table("listing_pricing_rule").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	t.Cleanup(SetOwnerScopeRequiredForTesting(true))
	ctx := withRequestIdentity(context.TODO(), "user-a", nil)

	rows, total, _, _, err := findPricingRuleRows(ctx, db.Table("listing_pricing_rule"), PricingRuleQuery{
		TenantID:   101,
		Name:       "Alpha",
		RuleCode:   "P-1",
		StoreID:    &storeID,
		CategoryID: &categoryID,
		RuleType:   "ratio",
		Status:     &status,
	})
	if err != nil {
		t.Fatalf("findPricingRuleRows: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].RuleCode != "P-1" {
		t.Fatalf("rows = %+v total=%d, want only fully matched rule", rows, total)
	}
}

func TestGormPricingRuleRepositoryListByStoreIDReturnsNewestFirst(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingPricingRule{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, row := range []listingPricingRule{
		{TenantID: 101, Name: "old", RuleCode: "P-1", StoreID: 11, RuleType: "ratio", RuleValue: 1.1, Status: 0, Deleted: 0},
		{TenantID: 101, Name: "new", RuleCode: "P-2", StoreID: 11, RuleType: "fixed", RuleValue: 2.2, Status: 0, Deleted: 0},
	} {
		if err := db.Table("listing_pricing_rule").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	repo := NewGormPricingRuleRepository(db)
	items, err := repo.ListByStoreID(context.Background(), 11)
	if err != nil {
		t.Fatalf("ListByStoreID() error = %v", err)
	}
	if len(items) != 2 || items[0].RuleCode != "P-2" || items[1].RuleCode != "P-1" {
		t.Fatalf("ListByStoreID() = %+v, want desc order by newest id", items)
	}
}
