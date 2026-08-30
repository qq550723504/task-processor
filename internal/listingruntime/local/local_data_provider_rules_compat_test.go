package local

import (
	"testing"
	"time"

	"task-processor/internal/listingadmin"
)

func TestLocalDataProviderRuleCompatibilityUsesResourceAPIs(t *testing.T) {
	now := time.Date(2026, time.August, 30, 14, 0, 0, 0, time.UTC)

	t.Run("filter", func(t *testing.T) {
		db := newLocalRuntimeFilterRuleTestDB(t)
		if err := db.Table("listing_filter_rule").Create(&localRuntimeFilterRuleRow{ID: 911, TenantID: 246, Name: "filter", RuleCode: "FILTER", StoreID: 986, PriceMin: 10.5, CreateTime: now}).Error; err != nil {
			t.Fatalf("seed filter rule: %v", err)
		}
		provider := NewLocalDataProviderFromResources(NewRuntimeResources(db, nil))
		rules, err := provider.GetFilterRule(&listingadmin.FilterRuleReqDTO{TenantID: 246, StoreID: 986})
		if err != nil || rules == nil || len(*rules) != 1 || (*rules)[0].ID != 911 || (*rules)[0].PriceMin == nil || *(*rules)[0].PriceMin != 10.5 {
			t.Fatalf("GetFilterRule() = %#v, %v; want resource-backed filter DTO", rules, err)
		}
	})

	t.Run("profit", func(t *testing.T) {
		db := newLocalRuntimeProfitRuleTestDB(t)
		if err := db.Table("listing_profit_rule").Create(&localRuntimeProfitRuleRow{ID: 1011, TenantID: 246, Name: "profit", RuleCode: "PROFIT", StoreID: 986, SalePriceMultiplier: 1.3, DiscountPriceMultiplier: 0.8, CreateTime: now}).Error; err != nil {
			t.Fatalf("seed profit rule: %v", err)
		}
		provider := NewLocalDataProviderFromResources(NewRuntimeResources(db, nil))
		rule, err := provider.GetProfitRule(&listingadmin.ProfitRuleReqDTO{TenantID: 246, StoreID: 986})
		if err != nil || rule == nil || rule.ID != 1011 || rule.SalePriceMultiplier != 1.3 {
			t.Fatalf("GetProfitRule() = %#v, %v; want resource-backed profit DTO", rule, err)
		}
	})

	t.Run("pricing", func(t *testing.T) {
		db := newLocalRuntimePricingRuleTestDB(t)
		if err := db.Table("listing_pricing_rule").Create(&localRuntimePricingRuleRow{ID: 811, TenantID: 246, Name: "pricing", RuleCode: "PRICING", StoreID: 986, RuleType: "ratio", RuleValue: 1.2, CreateTime: now}).Error; err != nil {
			t.Fatalf("seed pricing rule: %v", err)
		}
		provider := NewLocalDataProviderFromResources(NewRuntimeResources(db, nil))
		storeID := int64(986)
		rules, err := provider.GetPricingRule(&listingadmin.PricingRuleReqDTO{StoreID: &storeID})
		if err != nil || len(rules) != 1 || rules[0].ID != 811 || rules[0].RuleValue == nil || *rules[0].RuleValue != 1.2 {
			t.Fatalf("GetPricingRule() = %#v, %v; want resource-backed pricing DTO", rules, err)
		}
	})
}
