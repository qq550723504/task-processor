package skc

import (
	"context"
	"testing"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"
	sheinimage "task-processor/internal/shein/api/image"
	productapi "task-processor/internal/shein/api/product"
	sheinattr "task-processor/internal/shein/product/attribute"
)

func TestSKCVariantProcessor_NewSKURuntimeInputIncludesContextDependencies(t *testing.T) {
	ctx := shein.NewTaskContext(context.Background(), &model.Task{Region: "US"})
	fixedStock := 7

	ctx.StoreInfo = &managementapi.StoreRespDTO{FixedStockCount: &fixedStock}
	ctx.ProfitRule = &managementapi.ProfitRuleRespDTO{}
	ctx.SiteList = []productapi.SiteInfo{{SubSiteList: []string{"us"}}}
	ctx.ImageAPI = &sheinimage.Client{}

	amazonProduct := &model.Product{Title: "Test Product"}
	variants := []model.Product{{Asin: "A1"}}
	asinSkuMap := map[string]string{"A1": "SKU-1"}

	processor := &SKCVariantProcessor{
		runtime: &SKCRuntimeInput{
			Region:             "SG",
			AmazonProduct:      amazonProduct,
			Variants:           variants,
			AttributeTemplates: nil,
			AsinSkuMap:         asinSkuMap,
		},
	}

	runtime := processor.newSKURuntimeInput(ctx)
	if runtime.StoreInfo != ctx.StoreInfo {
		t.Fatal("expected store info to be copied from task context")
	}
	if runtime.ProfitRule != ctx.ProfitRule {
		t.Fatal("expected profit rule to be copied from task context")
	}
	if runtime.ImageAPI != ctx.ImageAPI {
		t.Fatal("expected image API to be copied from task context")
	}
	if len(runtime.SiteList) != 1 || len(runtime.SiteList[0].SubSiteList) != 1 || runtime.SiteList[0].SubSiteList[0] != "us" {
		t.Fatalf("expected site list to be copied from task context, got %+v", runtime.SiteList)
	}
	if runtime.Region != "SG" {
		t.Fatalf("expected runtime region to prefer SKC runtime value, got %q", runtime.Region)
	}
	if runtime.AmazonProduct != amazonProduct {
		t.Fatal("expected amazon product to be copied from SKC runtime")
	}
	if len(runtime.Variants) != 1 || runtime.Variants[0].Asin != "A1" {
		t.Fatalf("expected variants to be copied, got %+v", runtime.Variants)
	}
	if runtime.AsinSkuMap["A1"] != "SKU-1" {
		t.Fatalf("expected asin sku map to be copied, got %+v", runtime.AsinSkuMap)
	}
}

func TestValidateMappedStrategyAttributes_ReturnsErrorWhenSecondaryValuesRemainUnmapped(t *testing.T) {
	strategy := sheinattr.AttributeStrategy{
		PrimaryAttribute: sheinattr.ResultAttribute{
			AttrID: 27,
			AttrValue: []sheinattr.AttributeValue{
				{ID: 100, Value: "Light Pink"},
			},
		},
		SecondaryAttribute: sheinattr.ResultAttribute{
			AttrID: 87,
			AttrValue: []sheinattr.AttributeValue{
				{ID: 0, Value: "6"},
				{ID: 0, Value: "6.5"},
			},
		},
	}

	err := validateMappedStrategyAttributes(strategy)
	if err == nil {
		t.Fatal("expected error when all secondary attribute values remain unmapped")
	}
	if got := err.Error(); got != "secondary attribute 87 has no valid mapped values after ID mapping" {
		t.Fatalf("unexpected error: %v", err)
	}
}
