package local

import (
	"testing"
	"time"

	"task-processor/internal/listingadmin"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestLocalRuntimeProfitRuleClientUsesResourcesForTenantFallbackWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeProfitRuleTestDB(t)
	now := time.Date(2026, time.August, 30, 13, 0, 0, 0, time.UTC)
	if err := db.Table("listing_profit_rule").Create(&localRuntimeProfitRuleRow{
		ID:                      1001,
		TenantID:                246,
		Name:                    "Tenant default",
		RuleCode:                "DEFAULT",
		StoreID:                 0,
		SalePriceMultiplier:     1.35,
		DiscountPriceMultiplier: 0.9,
		Status:                  0,
		CreateTime:              now,
	}).Error; err != nil {
		t.Fatalf("seed profit rule: %v", err)
	}

	runtime := &LocalRuntime{resources: NewRuntimeResources(db, nil)}
	client := runtime.GetProfitRuleClient()
	if client == nil {
		t.Fatal("GetProfitRuleClient() returned nil")
	}
	rule, err := client.GetProfitRule(&listingadmin.ProfitRuleReqDTO{TenantID: 246, StoreID: 986})
	if err != nil {
		t.Fatalf("GetProfitRule() error: %v", err)
	}
	if rule == nil {
		t.Fatal("GetProfitRule() returned nil")
	}
	if rule.ID != 1001 || rule.RuleCode != "DEFAULT" || rule.StoreID != nil || rule.SalePriceMultiplier != 1.35 || rule.DiscountPriceMultiplier != 0.9 {
		t.Fatalf("GetProfitRule() = %#v, want tenant fallback DTO", rule)
	}
}

type localRuntimeProfitRuleRow struct {
	ID                      int64     `gorm:"column:id;primaryKey"`
	TenantID                int64     `gorm:"column:tenant_id"`
	Name                    string    `gorm:"column:name"`
	RuleCode                string    `gorm:"column:rule_code"`
	Description             string    `gorm:"column:description"`
	StoreID                 int64     `gorm:"column:store_id"`
	CategoryID              int64     `gorm:"column:category_id"`
	SalePriceMultiplier     float64   `gorm:"column:sale_price_multiplier"`
	DiscountPriceMultiplier float64   `gorm:"column:discount_price_multiplier"`
	Status                  int16     `gorm:"column:status"`
	Remark                  string    `gorm:"column:remark"`
	CreateTime              time.Time `gorm:"column:create_time"`
	Deleted                 int16     `gorm:"column:deleted"`
}

func newLocalRuntimeProfitRuleTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Table("listing_profit_rule").AutoMigrate(&localRuntimeProfitRuleRow{}); err != nil {
		t.Fatalf("migrate profit rule: %v", err)
	}
	return db
}
