package pricing

import (
	"context"
	"testing"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	sheinapi "task-processor/internal/shein/api/pricing"

	"github.com/sirupsen/logrus"
)

func TestGetPricingRulesPrefersRepository(t *testing.T) {
	service := &autoPricingServiceImpl{
		pricingRuleRepo: stubSheinPricingRuleRepo{
			rules: []listingadmin.PricingRule{
				{ID: 1, Name: "repo-rule", RuleType: "FIXED", RuleValue: 2, Status: 0},
			},
		},
		logger: logrus.NewEntry(logrus.New()),
	}

	rules, err := service.getPricingRules(9)
	if err != nil {
		t.Fatalf("getPricingRules() error = %v", err)
	}
	if len(rules) != 1 || rules[0].Name != "repo-rule" {
		t.Fatalf("getPricingRules() = %+v, want repository rules", rules)
	}
}

func TestCheckAllSKUsPassConditionPrefersRepository(t *testing.T) {
	service := &autoPricingServiceImpl{
		mappingRepo: stubSheinMappingRepo{
			mapping: &listingadmin.ProductImportMapping{
				PlatformProductID:   "SKU-1",
				ProductID:           "ASIN-1",
				SalePriceMultiplier: 2,
			},
		},
		calculator: NewAutoPricingCalculator(),
		logger:     logrus.NewEntry(logrus.New()),
	}

	rules := []listingruntime.PricingRule{
		{RuleType: "fixed", RuleValue: sheinTestFloat64Ptr(5)},
	}
	skus := []sheinapi.SkuCostPrice{
		{
			SkuCode:          "SKU-1",
			SuggestCostPrice: 60,
			CostPriceHistories: []sheinapi.CostPriceHistory{
				{CostPrice: 100},
			},
		},
	}

	allPass, shouldSkip := service.checkAllSKUsPassCondition(skus, rules, 8)
	if !allPass || shouldSkip {
		t.Fatalf("checkAllSKUsPassCondition() = (%v, %v), want (true, false)", allPass, shouldSkip)
	}
}

type stubSheinPricingRuleRepo struct {
	rules []listingadmin.PricingRule
}

func (s stubSheinPricingRuleRepo) ListByStoreID(_ context.Context, _ int64) ([]listingadmin.PricingRule, error) {
	return s.rules, nil
}

type stubSheinMappingRepo struct {
	mapping *listingadmin.ProductImportMapping
}

func (s stubSheinMappingRepo) FindLatest(_ context.Context, _ listingadmin.ProductImportMappingQuery) (*listingadmin.ProductImportMapping, error) {
	return s.mapping, nil
}

func sheinTestFloat64Ptr(v float64) *float64 { return &v }
