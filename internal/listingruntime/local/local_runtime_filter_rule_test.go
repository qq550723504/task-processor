package local

import (
	"testing"
	"time"

	"task-processor/internal/listingadmin"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestLocalRuntimeFilterRuleClientUsesResourcesForStoreFallbackWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeFilterRuleTestDB(t)
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	for _, rule := range []localRuntimeFilterRuleRow{
		{
			ID: 901, TenantID: 246, Name: "Store fallback", RuleCode: "STORE", StoreID: 986,
			PriceType: "fixed", PriceMin: 11.5, PriceMax: 99.5, StockMin: 3, Status: 0, CreateTime: now,
		},
		{
			ID: 902, TenantID: 246, Name: "Tenant fallback", RuleCode: "TENANT", StoreID: 0,
			PriceType: "fixed", PriceMin: 1.5, PriceMax: 9.5, StockMin: 1, Status: 0, CreateTime: now,
		},
	} {
		if err := db.Table("listing_filter_rule").Create(&rule).Error; err != nil {
			t.Fatalf("seed filter rule: %v", err)
		}
	}

	runtime := &LocalRuntime{resources: NewRuntimeResources(db, nil)}
	client := runtime.GetFilterRuleClient()
	if client == nil {
		t.Fatal("GetFilterRuleClient() returned nil")
	}
	rules, err := client.GetFilterRule(&listingadmin.FilterRuleReqDTO{TenantID: 246, StoreID: 986, CategoryID: 12})
	if err != nil {
		t.Fatalf("GetFilterRule() error: %v", err)
	}
	if rules == nil || len(*rules) != 1 {
		t.Fatalf("GetFilterRule() = %#v, want one store fallback rule", rules)
	}
	if (*rules)[0].ID != 901 || (*rules)[0].RuleCode != "STORE" || (*rules)[0].StoreID != 986 || (*rules)[0].PriceMin == nil || *(*rules)[0].PriceMin != 11.5 {
		t.Fatalf("GetFilterRule() = %#v, want store fallback DTO", (*rules)[0])
	}
}

type localRuntimeFilterRuleRow struct {
	ID              int64     `gorm:"column:id;primaryKey"`
	TenantID        int64     `gorm:"column:tenant_id"`
	Name            string    `gorm:"column:name"`
	RuleCode        string    `gorm:"column:rule_code"`
	Description     string    `gorm:"column:description"`
	StoreID         int64     `gorm:"column:store_id"`
	CategoryID      int64     `gorm:"column:category_id"`
	PriceType       string    `gorm:"column:price_type"`
	PriceMin        float64   `gorm:"column:price_min"`
	PriceMax        float64   `gorm:"column:price_max"`
	StockMin        int       `gorm:"column:stock_min"`
	RatingMin       float64   `gorm:"column:rating_min"`
	ReviewCountMin  int       `gorm:"column:review_count_min"`
	DeliveryTimeMax int       `gorm:"column:delivery_time_max"`
	FulfillmentType string    `gorm:"column:fulfillment_type"`
	Status          int16     `gorm:"column:status"`
	Remark          string    `gorm:"column:remark"`
	CreateTime      time.Time `gorm:"column:create_time"`
	Deleted         int16     `gorm:"column:deleted"`
}

func newLocalRuntimeFilterRuleTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Table("listing_filter_rule").AutoMigrate(&localRuntimeFilterRuleRow{}); err != nil {
		t.Fatalf("migrate filter rule: %v", err)
	}
	return db
}
