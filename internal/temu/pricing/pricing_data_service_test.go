package pricing

import (
	"context"
	"testing"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	managementapi "task-processor/internal/ports/managementapi"

	"github.com/sirupsen/logrus"
)

func TestGetPricingRulesPrefersRepository(t *testing.T) {
	service := &PricingDataService{
		pricingRuleRepo: stubPricingRuleRepo{
			rules: []listingadmin.PricingRule{
				{ID: 1, Name: "enabled", RuleType: "MULTIPLY", RuleValue: 1.5, Status: 0},
				{ID: 2, Name: "disabled", RuleType: "MULTIPLY", RuleValue: 1.2, Status: 1},
			},
		},
		logger: logrus.NewEntry(logrus.New()),
	}

	rules, err := service.GetPricingRules(10)
	if err != nil {
		t.Fatalf("GetPricingRules() error = %v", err)
	}
	if len(rules) != 1 || rules[0].Name != "enabled" {
		t.Fatalf("GetPricingRules() = %+v, want only enabled repository rule", rules)
	}
}

func TestGetProductImportMappingPrefersRepository(t *testing.T) {
	service := &PricingDataService{
		mappingRepo: stubProductImportMappingRepo{
			mapping: &listingadmin.ProductImportMapping{
				ID:                  1,
				StoreID:             8,
				ProductID:           "ASIN-1",
				SKU:                 "SKU-1",
				SalePriceMultiplier: 1.8,
				Status:              0,
			},
		},
		logger:  logrus.NewEntry(logrus.New()),
		runtime: NewManagementRuntime(stubPricingRuntimeSource{}),
	}

	mapping, err := service.GetProductImportMapping("SKU-1", 8)
	if err != nil {
		t.Fatalf("GetProductImportMapping() error = %v", err)
	}
	if mapping == nil || mapping.ProductId != "ASIN-1" {
		t.Fatalf("GetProductImportMapping() = %+v, want repository mapping", mapping)
	}
}

type stubPricingRuleRepo struct {
	rules []listingadmin.PricingRule
}

func (s stubPricingRuleRepo) ListByStoreID(_ context.Context, _ int64) ([]listingadmin.PricingRule, error) {
	return s.rules, nil
}

type stubProductImportMappingRepo struct {
	mapping *listingadmin.ProductImportMapping
}

func (s stubProductImportMappingRepo) FindLatest(_ context.Context, _ listingadmin.ProductImportMappingQuery) (*listingadmin.ProductImportMapping, error) {
	return s.mapping, nil
}

var _ = managementapi.PricingRuleRespDTO{}

type stubPricingRuntimeSource struct{}

func (stubPricingRuntimeSource) GetStoreAPI() managementapi.StoreAPI { return nil }

func (stubPricingRuntimeSource) GetPricingRuleClient() managementapi.PricingRuleAPI { return nil }

func (stubPricingRuntimeSource) GetProductImportMappingAPI() managementapi.ProductImportMappingAPI {
	return nil
}

func (stubPricingRuntimeSource) GetLocalStoreRepository() *listingadmin.GormStoreRepository {
	return nil
}

func (stubPricingRuntimeSource) GetLocalPricingRuleRepository() *listingadmin.GormPricingRuleRepository {
	return nil
}

func (stubPricingRuntimeSource) GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository {
	return nil
}

func (stubPricingRuntimeSource) GetRuntimeOperationStrategy(int64) (*listingruntime.OperationStrategy, error) {
	return nil, nil
}
