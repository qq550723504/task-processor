package sale

import (
	"testing"

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
