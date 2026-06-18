package sku

import (
	"context"
	"testing"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	"task-processor/internal/pkg/types"
	shein "task-processor/internal/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheinproduct "task-processor/internal/shein/api/product"
	"task-processor/internal/shein/product/variant"
)

type stubImageAPI struct{}

func (stubImageAPI) UploadOriginalImage(_ []byte) (string, error)    { return "", nil }
func (stubImageAPI) DownloadAndUploadImage(_ string) (string, error) { return "", nil }

func TestBuildSKUListForMultipleVariantsWithRuntimeDeduplicatesSupplierSKU(t *testing.T) {
	builder := NewSKUStrategyProcessor(nil)

	runtime := &RuntimeInput{
		AmazonProduct: &model.Product{
			Asin:       "ASIN-1",
			FinalPrice: 19.99,
		},
		Variants: []model.Product{{
			Asin:       "ASIN-1",
			FinalPrice: 19.99,
		}},
		AsinSkuMap: map[string]string{
			"ASIN-1": "SUPPLIER-SKU-1",
		},
		AttributeTemplates: &sheinattribute.AttributeTemplateInfo{},
		StoreInfo: &managementapi.StoreRespDTO{
			PriceType: "special",
		},
		ProfitRule: &managementapi.ProfitRuleRespDTO{
			SalePriceMultiplier: 1,
		},
		SiteList: []sheinproduct.SiteInfo{{
			SubSiteList: []string{"us"},
		}},
		Region:   "US",
		ImageAPI: stubImageAPI{},
	}

	req := shein.SKUBuildRequest{
		WarehouseCode: "W1",
		Strategy: shein.AttributeStrategy{
			PrimaryAttribute: shein.ResultAttribute{AttrID: 27},
		},
	}

	variant := shein.Variant{
		ASIN:       "ASIN-1",
		Attributes: map[string]string{"Color": "Black", "Size": "7"},
		Length:     types.FlexibleString("1"),
		Width:      types.FlexibleString("1"),
		Height:     types.FlexibleString("1"),
		Weight:     types.FlexibleString("100g"),
		LengthUnit: "cm",
	}

	variantInfoMap := map[string]shein.VariantInfo{
		"ASIN-1:1001": {
			Variant: variant,
			AttrID:  87,
			ValueID: 1001,
		},
		"ASIN-1:1002": {
			Variant: variant,
			AttrID:  87,
			ValueID: 1002,
		},
	}

	skus, err := builder.buildSKUListForMultipleVariantsWithRuntime(nil, runtime, variantInfoMap, req)
	if err != nil {
		t.Fatalf("buildSKUListForMultipleVariantsWithRuntime() error = %v", err)
	}
	if len(skus) != 1 {
		t.Fatalf("expected 1 deduplicated SKU, got %d", len(skus))
	}
	if skus[0].SupplierSKU != "SUPPLIER-SKU-1" {
		t.Fatalf("expected supplier sku SUPPLIER-SKU-1, got %q", skus[0].SupplierSKU)
	}
}

func TestBuildMultipleSKUsWithRuntimeUsesPreMatchedPrimaryVariants(t *testing.T) {
	builder := NewSKUStrategyProcessor(variant.NewVariantMatcher())
	ctx := shein.NewTaskContext(context.Background(), nil)
	ctx.AttributeTemplates = &sheinattribute.AttributeTemplateInfo{
		Data: []sheinattribute.AttributeTemplate{
			{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{AttributeID: 27, AttributeName: "颜色", AttributeNameEn: "Color"},
					{AttributeID: 87, AttributeName: "尺寸", AttributeNameEn: "Size"},
				},
			},
		},
	}

	runtime := &RuntimeInput{
		AmazonProduct: &model.Product{
			Asin:       "ASIN-WHITE-1",
			FinalPrice: 19.99,
		},
		Variants: []model.Product{
			{Asin: "ASIN-WHITE-1", FinalPrice: 19.99},
			{Asin: "ASIN-WHITE-2", FinalPrice: 20.99},
		},
		AsinSkuMap: map[string]string{
			"ASIN-WHITE-1": "SUPPLIER-WHITE-1",
			"ASIN-WHITE-2": "SUPPLIER-WHITE-2",
		},
		AttributeTemplates: &sheinattribute.AttributeTemplateInfo{},
		StoreInfo: &managementapi.StoreRespDTO{
			PriceType: "special",
		},
		ProfitRule: &managementapi.ProfitRuleRespDTO{
			SalePriceMultiplier: 1,
		},
		SiteList: []sheinproduct.SiteInfo{{
			SubSiteList: []string{"us"},
		}},
		Region:   "US",
		ImageAPI: stubImageAPI{},
	}

	fullVariants := []shein.Variant{
		{
			ASIN:       "ASIN-WHITE-1",
			Attributes: map[string]string{"Color": "White", "Size": "10"},
			Length:     types.FlexibleString("1"),
			Width:      types.FlexibleString("1"),
			Height:     types.FlexibleString("1"),
			Weight:     types.FlexibleString("100g"),
			LengthUnit: "cm",
		},
		{
			ASIN:       "ASIN-WHITE-2",
			Attributes: map[string]string{"Color": "White", "Size": "10.5"},
			Length:     types.FlexibleString("1"),
			Width:      types.FlexibleString("1"),
			Height:     types.FlexibleString("1"),
			Weight:     types.FlexibleString("100g"),
			LengthUnit: "cm",
		},
	}
	whiteOnly := []shein.Variant{fullVariants[0]}

	req := shein.SKUBuildRequest{
		SaleAttributeData: shein.ResultSaleAttribute{
			Variants: fullVariants,
		},
		Strategy: shein.AttributeStrategy{
			PrimaryAttribute: shein.ResultAttribute{
				AttrID: 27,
			},
			SecondaryAttribute: shein.ResultAttribute{
				AttrID: 87,
				AttrValue: []shein.AttributeValue{
					{ID: types.FlexibleID(1001), Value: "10"},
					{ID: types.FlexibleID(1002), Value: "10.5"},
				},
			},
		},
		PrimaryAttrValue: "White",
		WarehouseCode:    "W1",
		MatchedVariants:  whiteOnly,
	}

	skus, err := builder.BuildMultipleSKUsWithRuntime(ctx, runtime, req)
	if err != nil {
		t.Fatalf("BuildMultipleSKUsWithRuntime() error = %v", err)
	}
	if len(skus) != 1 {
		t.Fatalf("expected 1 sku from pre-matched primary variants, got %d", len(skus))
	}
	if skus[0].SupplierSKU != "SUPPLIER-WHITE-1" {
		t.Fatalf("expected supplier sku SUPPLIER-WHITE-1, got %q", skus[0].SupplierSKU)
	}
}
