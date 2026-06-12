package skc

import (
	"context"
	"testing"

	sheinattribute "task-processor/internal/shein/api/attribute"
	sheinctx "task-processor/internal/shein/context"
)

func TestBuildStrategyFromSelectionUsesResolvedPrimaryAndSecondaryDimensions(t *testing.T) {
	t.Parallel()

	ctx := sheinctx.NewTaskContext(context.Background(), nil)
	ctx.SetSaleAttributeSelection(&sheinctx.SaleAttributeSelectionState{
		Source:                   "resolution",
		PrimaryAttributeID:       27,
		SecondaryAttributeID:     87,
		PrimarySourceDimension:   "Color",
		SecondarySourceDimension: "Size",
	})
	saleSpec := &sheinctx.ResultSaleAttribute{
		SaleAttributes: []sheinctx.ResultAttribute{
			{AttrID: 27, AttrValue: []sheinctx.AttributeValue{{ID: 11, Value: "Red"}, {ID: 12, Value: "Blue"}}},
			{AttrID: 87, AttrValue: []sheinctx.AttributeValue{{ID: 21, Value: "S"}, {ID: 22, Value: "M"}}},
		},
		Variants: []sheinctx.Variant{
			{ASIN: "A1", Attributes: map[string]string{"Color": "Red", "Size": "S"}},
			{ASIN: "A2", Attributes: map[string]string{"Color": "Blue", "Size": "M"}},
		},
	}

	strategy, adapted, source, err := BuildStrategyFromSelection(ctx, saleSpec, makeSelectionTemplates())
	if err != nil {
		t.Fatalf("BuildStrategyFromSelection() error = %v", err)
	}
	if source != "selection" {
		t.Fatalf("source = %q, want selection", source)
	}
	if strategy.PrimaryAttribute.AttrID != 27 {
		t.Fatalf("primary attr id = %d, want 27", strategy.PrimaryAttribute.AttrID)
	}
	if strategy.SecondaryAttribute.AttrID != 87 {
		t.Fatalf("secondary attr id = %d, want 87", strategy.SecondaryAttribute.AttrID)
	}
	if len(adapted.Variants) != 2 {
		t.Fatalf("adapted variants = %d, want 2", len(adapted.Variants))
	}
}

func TestBuildStrategyFromSelectionSynthesizesTemplateAttributeFromSourceDimension(t *testing.T) {
	t.Parallel()

	ctx := sheinctx.NewTaskContext(context.Background(), nil)
	ctx.SetSaleAttributeSelection(&sheinctx.SaleAttributeSelectionState{
		Source:                   "resolution",
		PrimaryAttributeID:       1001184,
		SecondaryAttributeID:     87,
		PrimarySourceDimension:   "Style",
		SecondarySourceDimension: "Size",
	})
	saleSpec := &sheinctx.ResultSaleAttribute{
		SaleAttributes: []sheinctx.ResultAttribute{
			{AttrID: 27, AttrValue: []sheinctx.AttributeValue{{ID: 11, Value: "White"}}},
			{AttrID: 87, AttrValue: []sheinctx.AttributeValue{{ID: 21, Value: "S"}, {ID: 22, Value: "M"}}},
		},
		Variants: []sheinctx.Variant{
			{ASIN: "A1", Attributes: map[string]string{"Color": "White", "Style": "Bandana", "Size": "S"}},
			{ASIN: "A2", Attributes: map[string]string{"Color": "White", "Style": "Bow", "Size": "M"}},
		},
	}

	strategy, adapted, source, err := BuildStrategyFromSelection(ctx, saleSpec, makeSelectionTemplates())
	if err != nil {
		t.Fatalf("BuildStrategyFromSelection() error = %v", err)
	}
	if source != "selection" {
		t.Fatalf("source = %q, want selection", source)
	}
	if strategy.PrimaryAttribute.AttrID != 1001184 {
		t.Fatalf("primary attr id = %d, want 1001184", strategy.PrimaryAttribute.AttrID)
	}
	if len(strategy.PrimaryAttribute.AttrValue) != 2 {
		t.Fatalf("primary attr values = %d, want 2", len(strategy.PrimaryAttribute.AttrValue))
	}
	if strategy.PrimaryAttribute.AttrValue[0].ID.Int() >= 0 {
		t.Fatalf("expected synthesized primary value ids to be temporary negatives, got %d", strategy.PrimaryAttribute.AttrValue[0].ID.Int())
	}
	if got := adapted.Variants[0].Attributes["Style Type"]; got != "Bandana" {
		t.Fatalf("adapted variant style type = %q, want Bandana", got)
	}
	if got := adapted.Variants[1].Attributes["Style Type"]; got != "Bow" {
		t.Fatalf("adapted variant style type = %q, want Bow", got)
	}
}

func TestBuildStrategyFromSelectionIgnoresStaleExistingAttributeValuesWhenSourceDimensionDiffers(t *testing.T) {
	t.Parallel()

	ctx := sheinctx.NewTaskContext(context.Background(), nil)
	ctx.SetSaleAttributeSelection(&sheinctx.SaleAttributeSelectionState{
		Source:                 "resolution",
		PrimaryAttributeID:     27,
		PrimarySourceDimension: "Style",
	})
	saleSpec := &sheinctx.ResultSaleAttribute{
		SaleAttributes: []sheinctx.ResultAttribute{
			{AttrID: 27, AttrValue: []sheinctx.AttributeValue{{ID: 11, Value: "White"}}},
		},
		Variants: []sheinctx.Variant{
			{ASIN: "A1", Attributes: map[string]string{"Style": "Bandana"}},
			{ASIN: "A2", Attributes: map[string]string{"Style": "Bow"}},
		},
	}

	strategy, _, _, err := BuildStrategyFromSelection(ctx, saleSpec, makeSelectionTemplates())
	if err != nil {
		t.Fatalf("BuildStrategyFromSelection() error = %v", err)
	}
	if len(strategy.PrimaryAttribute.AttrValue) != 2 {
		t.Fatalf("primary attr values = %d, want 2", len(strategy.PrimaryAttribute.AttrValue))
	}
	if strategy.PrimaryAttribute.AttrValue[0].Value != "Bandana" {
		t.Fatalf("first primary attr value = %q, want Bandana", strategy.PrimaryAttribute.AttrValue[0].Value)
	}
}

func TestBuildStrategyFromSelectionFallsBackWhenTemplateAttributeNamesAreUnavailable(t *testing.T) {
	t.Parallel()

	ctx := sheinctx.NewTaskContext(context.Background(), nil)
	ctx.SetSaleAttributeSelection(&sheinctx.SaleAttributeSelectionState{
		Source:                 "resolution",
		PrimaryAttributeID:     9999,
		PrimarySourceDimension: "Style",
	})
	saleSpec := &sheinctx.ResultSaleAttribute{
		Variants: []sheinctx.Variant{
			{ASIN: "A1", Attributes: map[string]string{"Style": "Bandana"}},
		},
	}

	if _, _, _, err := BuildStrategyFromSelection(ctx, saleSpec, makeSelectionTemplates()); err == nil {
		t.Fatal("expected error when template attribute names are unavailable")
	}
}

func makeSelectionTemplates() *sheinattribute.AttributeTemplateInfo {
	return &sheinattribute.AttributeTemplateInfo{
		Data: []sheinattribute.AttributeTemplate{{
			AttributeInfos: []sheinattribute.AttributeInfo{
				{AttributeID: 27, AttributeName: "颜色", AttributeNameEn: "Color", AttributeType: 1},
				{AttributeID: 87, AttributeName: "尺寸", AttributeNameEn: "Size", AttributeType: 1},
				{AttributeID: 1001184, AttributeName: "款式", AttributeNameEn: "Style Type", AttributeType: 1},
			},
		}},
	}
}
