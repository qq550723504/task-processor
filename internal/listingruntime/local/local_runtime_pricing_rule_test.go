package local

import (
	"testing"
	"time"

	"task-processor/internal/listingadmin"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestLocalRuntimePricingRuleClientUsesResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimePricingRuleTestDB(t)
	now := time.Date(2026, time.August, 30, 11, 0, 0, 0, time.UTC)
	for _, rule := range []localRuntimePricingRuleRow{
		{
			ID: 801, TenantID: 246, Name: "Target store", RuleCode: "TARGET", StoreID: 986,
			RuleType: "ratio", RuleValue: 1.2, Status: 1, CreateTime: now, Deleted: 0,
		},
		{
			ID: 802, TenantID: 246, Name: "Other store", RuleCode: "OTHER", StoreID: 987,
			RuleType: "ratio", RuleValue: 1.5, Status: 1, CreateTime: now, Deleted: 0,
		},
	} {
		if err := db.Table("listing_pricing_rule").Create(&rule).Error; err != nil {
			t.Fatalf("seed pricing rule: %v", err)
		}
	}

	runtime := &LocalRuntime{resources: NewRuntimeResources(db, nil)}
	client := runtime.GetPricingRuleClient()
	if client == nil {
		t.Fatal("GetPricingRuleClient() returned nil")
	}
	storeID := int64(986)
	rules, err := client.GetPricingRule(&listingadmin.PricingRuleReqDTO{StoreID: &storeID})
	if err != nil {
		t.Fatalf("GetPricingRule() error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("GetPricingRule() returned %d rules, want 1", len(rules))
	}
	if rules[0].ID != 801 || rules[0].RuleCode != "TARGET" || rules[0].StoreID == nil || *rules[0].StoreID != 986 || rules[0].RuleValue == nil || *rules[0].RuleValue != 1.2 {
		t.Fatalf("GetPricingRule() = %#v, want target-store DTO", rules[0])
	}
}

type localRuntimePricingRuleRow struct {
	ID              int64     `gorm:"column:id;primaryKey"`
	TenantID        int64     `gorm:"column:tenant_id"`
	Name            string    `gorm:"column:name"`
	RuleCode        string    `gorm:"column:rule_code"`
	Description     string    `gorm:"column:description"`
	Remark          string    `gorm:"column:remark"`
	StoreID         int64     `gorm:"column:store_id"`
	CategoryID      int64     `gorm:"column:category_id"`
	PriceMin        float64   `gorm:"column:price_min"`
	PriceMax        float64   `gorm:"column:price_max"`
	RuleType        string    `gorm:"column:rule_type"`
	RuleValue       float64   `gorm:"column:rule_value"`
	FixedValue      float64   `gorm:"column:fixed_value"`
	AcceptCondition string    `gorm:"column:accept_condition"`
	RejectCondition string    `gorm:"column:reject_condition"`
	Status          int16     `gorm:"column:status"`
	CreateTime      time.Time `gorm:"column:create_time"`
	Deleted         int16     `gorm:"column:deleted"`
}

func newLocalRuntimePricingRuleTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Table("listing_pricing_rule").AutoMigrate(&localRuntimePricingRuleRow{}); err != nil {
		t.Fatalf("migrate pricing rule: %v", err)
	}
	return db
}
