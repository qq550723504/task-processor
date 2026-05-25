package listingadmin

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestFindProfitRuleRowsUsesRequestOwnerScope(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingProfitRule{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, row := range []listingProfitRule{
		{TenantID: 101, OwnerUserID: "user-a", Name: "keep", RuleCode: "PR-1", Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-b", Name: "other-owner", RuleCode: "PR-2", Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", Name: "deleted", RuleCode: "PR-3", Status: 1, Deleted: 1},
	} {
		if err := db.Table("listing_profit_rule").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	t.Cleanup(SetOwnerScopeRequiredForTesting(true))
	ctx := withRequestIdentity(context.TODO(), "user-a", nil)

	rows, total, page, pageSize, err := findProfitRuleRows(ctx, db.Table("listing_profit_rule"), ProfitRuleQuery{
		TenantID: 101,
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("findProfitRuleRows: %v", err)
	}
	if total != 1 || page != 1 || pageSize != 20 {
		t.Fatalf("result meta = total:%d page:%d pageSize:%d, want 1/1/20", total, page, pageSize)
	}
	if len(rows) != 1 || rows[0].Name != "keep" {
		t.Fatalf("rows = %+v, want only active owner-scoped row", rows)
	}
}

func TestFindProfitRuleRowsAppliesResourceFilters(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingProfitRule{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	storeID := int64(11)
	categoryID := int64(22)
	status := int16(2)
	for _, row := range []listingProfitRule{
		{TenantID: 101, OwnerUserID: "user-a", Name: "Alpha", RuleCode: "PR-1", StoreID: 11, CategoryID: 22, Status: 2, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", Name: "Alpha", RuleCode: "PR-2", StoreID: 11, CategoryID: 22, Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", Name: "Beta", RuleCode: "PR-3", StoreID: 12, CategoryID: 22, Status: 2, Deleted: 0},
	} {
		if err := db.Table("listing_profit_rule").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	t.Cleanup(SetOwnerScopeRequiredForTesting(true))
	ctx := withRequestIdentity(context.TODO(), "user-a", nil)

	rows, total, _, _, err := findProfitRuleRows(ctx, db.Table("listing_profit_rule"), ProfitRuleQuery{
		TenantID:   101,
		Name:       "Alpha",
		RuleCode:   "PR-1",
		StoreID:    &storeID,
		CategoryID: &categoryID,
		Status:     &status,
	})
	if err != nil {
		t.Fatalf("findProfitRuleRows: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].RuleCode != "PR-1" {
		t.Fatalf("rows = %+v total=%d, want only fully matched rule", rows, total)
	}
}
