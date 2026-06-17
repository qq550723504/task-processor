package sale

import (
	"testing"

	"task-processor/internal/model"
	"task-processor/internal/pkg/types"
	sheinattr "task-processor/internal/shein/product/attribute"
)

func TestRepairVariantWeights_BackfillsFromSourceProductWeight(t *testing.T) {
	handler := &SaleAttributeHandler{}
	data := sheinattr.ResultSaleAttribute{
		Variants: []sheinattr.Variant{
			{ASIN: "A1", Weight: types.FlexibleString("")},
			{ASIN: "A2", Weight: types.FlexibleString("430")},
		},
	}
	products := []map[string]string{
		{"asin": "A1", "weight": "250g"},
		{"asin": "A2", "weight": "430g"},
	}

	fixed := handler.repairVariantWeights(data, products)
	if got := fixed.Variants[0].Weight.String(); got != "250" {
		t.Fatalf("variant A1 weight = %q, want %q", got, "250")
	}
	if got := fixed.Variants[1].Weight.String(); got != "430" {
		t.Fatalf("variant A2 weight = %q, want %q", got, "430")
	}
}

func TestRepairVariantWeights_UsesFallbackWeightWhenVariantSourceMissing(t *testing.T) {
	handler := &SaleAttributeHandler{}
	data := sheinattr.ResultSaleAttribute{
		Variants: []sheinattr.Variant{
			{ASIN: "A1", Weight: types.FlexibleString("")},
			{ASIN: "A2", Weight: types.FlexibleString("400")},
		},
	}
	products := []map[string]string{
		{"asin": "A2", "weight": "0.43kg"},
	}

	fixed := handler.repairVariantWeights(data, products)
	if got := fixed.Variants[0].Weight.String(); got != "430" {
		t.Fatalf("variant A1 weight = %q, want %q", got, "430")
	}
}

func TestRepairVariantWeights_ParsesAmazonWeightTextFormats(t *testing.T) {
	handler := &SaleAttributeHandler{}
	data := sheinattr.ResultSaleAttribute{
		Variants: []sheinattr.Variant{
			{ASIN: "A1", Weight: types.FlexibleString("")},
		},
	}
	products := []map[string]string{
		{"asin": "A1", "weight": "3.52 ounces"},
	}

	fixed := handler.repairVariantWeights(data, products)
	if got := fixed.Variants[0].Weight.String(); got != "99.79" {
		t.Fatalf("variant A1 weight = %q, want %q", got, "99.79")
	}
}

func TestValidateAttributeValueConsistency_RemovesCrossDimensionPollution(t *testing.T) {
	handler := &SaleAttributeHandler{}
	amazonProduct := model.Product{
		VariationsValues: []model.VariationValue{
			{VariantName: "Size", Values: []string{"6", "6.5", "7"}},
			{VariantName: "Color", Values: []string{"Black", "Light Pink"}},
		},
	}
	data := sheinattr.ResultSaleAttribute{
		SaleAttributes: []sheinattr.ResultAttribute{
			{
				AttrID: 87,
				AttrValue: []sheinattr.AttributeValue{
					{ID: -1, Value: "6"},
					{ID: -1, Value: "Solid"},
					{ID: -1, Value: "6.5"},
				},
			},
			{
				AttrID: 27,
				AttrValue: []sheinattr.AttributeValue{
					{ID: -1, Value: "Black"},
					{ID: -1, Value: "Light Pink"},
				},
			},
		},
		Variants: []sheinattr.Variant{
			{ASIN: "A1", Attributes: map[string]string{"Color": "Black", "Size": "6"}},
			{ASIN: "A2", Attributes: map[string]string{"Color": "Light Pink", "Size": "Solid"}},
		},
	}

	fixed := handler.validateAttributeValueConsistency(amazonProduct, data)

	if got := fixed.Variants[1].Attributes["Size"]; got != "" {
		t.Fatalf("variant A2 size = %q, want empty after removing cross-dimension pollution", got)
	}

	gotValues := make([]string, 0, len(fixed.SaleAttributes[0].AttrValue))
	for _, item := range fixed.SaleAttributes[0].AttrValue {
		gotValues = append(gotValues, item.Value)
	}
	if len(gotValues) != 2 || gotValues[0] != "6" || gotValues[1] != "6.5" {
		t.Fatalf("size attr values = %#v, want [6 6.5]", gotValues)
	}
}
