package listingadmin

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestFindFilterRuleRowsUsesRequestOwnerScope(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingFilterRule{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, row := range []listingFilterRule{
		{TenantID: 101, OwnerUserID: "user-a", Name: "keep", RuleCode: "FR-1", Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-b", Name: "other-owner", RuleCode: "FR-2", Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", Name: "deleted", RuleCode: "FR-3", Status: 1, Deleted: 1},
	} {
		if err := db.Table("listing_filter_rule").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	t.Cleanup(SetOwnerScopeRequiredForTesting(true))
	ctx := withRequestIdentity(context.TODO(), "user-a", nil)

	rows, total, page, pageSize, err := findFilterRuleRows(ctx, db.Table("listing_filter_rule"), FilterRuleQuery{
		TenantID: 101,
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("findFilterRuleRows: %v", err)
	}
	if total != 1 || page != 1 || pageSize != 20 {
		t.Fatalf("result meta = total:%d page:%d pageSize:%d, want 1/1/20", total, page, pageSize)
	}
	if len(rows) != 1 || rows[0].Name != "keep" {
		t.Fatalf("rows = %+v, want only active owner-scoped row", rows)
	}
}

func TestFindFilterRuleRowsAppliesResourceFilters(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingFilterRule{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	storeID := int64(11)
	categoryID := int64(22)
	status := int16(2)
	for _, row := range []listingFilterRule{
		{TenantID: 101, OwnerUserID: "user-a", Name: "Alpha", RuleCode: "FR-1", StoreID: 11, CategoryID: 22, PriceType: "special", FulfillmentType: "FBA", Status: 2, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", Name: "Alpha", RuleCode: "FR-2", StoreID: 11, CategoryID: 22, PriceType: "normal", FulfillmentType: "FBA", Status: 2, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", Name: "Beta", RuleCode: "FR-3", StoreID: 12, CategoryID: 22, PriceType: "special", FulfillmentType: "FBM", Status: 2, Deleted: 0},
	} {
		if err := db.Table("listing_filter_rule").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	t.Cleanup(SetOwnerScopeRequiredForTesting(true))
	ctx := withRequestIdentity(context.TODO(), "user-a", nil)

	rows, total, _, _, err := findFilterRuleRows(ctx, db.Table("listing_filter_rule"), FilterRuleQuery{
		TenantID:        101,
		Name:            "Alpha",
		RuleCode:        "FR-1",
		StoreID:         &storeID,
		CategoryID:      &categoryID,
		PriceType:       "special",
		FulfillmentType: "FBA",
		Status:          &status,
	})
	if err != nil {
		t.Fatalf("findFilterRuleRows: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].RuleCode != "FR-1" {
		t.Fatalf("rows = %+v total=%d, want only fully matched rule", rows, total)
	}
}

func TestGormFilterRuleRepositoryResolveFilterRulesPrefersSpecificScope(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingFilterRule{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, row := range []listingFilterRule{
		{TenantID: 101, Name: "global", RuleCode: "FR-GLOBAL", Status: 1, Deleted: 0},
		{TenantID: 101, Name: "store", RuleCode: "FR-STORE", StoreID: 11, Status: 1, Deleted: 0},
		{TenantID: 101, Name: "category", RuleCode: "FR-CATEGORY", StoreID: 11, CategoryID: 22, Status: 1, Deleted: 0},
	} {
		if err := db.Table("listing_filter_rule").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	repo := NewGormFilterRuleRepository(db)
	items, err := repo.ResolveFilterRules(context.Background(), 101, 11, 22)
	if err != nil {
		t.Fatalf("ResolveFilterRules() error = %v", err)
	}
	if len(items) != 1 || items[0].RuleCode != "FR-CATEGORY" {
		t.Fatalf("category items = %+v, want category-specific rule", items)
	}

	items, err = repo.ResolveFilterRules(context.Background(), 101, 11, 99)
	if err != nil {
		t.Fatalf("ResolveFilterRules() store fallback error = %v", err)
	}
	if len(items) != 1 || items[0].RuleCode != "FR-STORE" {
		t.Fatalf("store fallback items = %+v, want store-specific rule", items)
	}

	items, err = repo.ResolveFilterRules(context.Background(), 101, 88, 99)
	if err != nil {
		t.Fatalf("ResolveFilterRules() global fallback error = %v", err)
	}
	if len(items) != 1 || items[0].RuleCode != "FR-GLOBAL" {
		t.Fatalf("global fallback items = %+v, want global rule", items)
	}
}
