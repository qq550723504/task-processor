package shein

import "testing"

func TestMatchSubmitVariantOptionIndexPrefersSourceSKU(t *testing.T) {
	t.Parallel()

	input := &SubmitVariantContext{
		Variants: []SubmitVariantOption{
			{VariantSKU: "SKU-A", Color: "black", Size: "L"},
			{VariantSKU: "SKU-B", Color: "white", Size: "M"},
		},
	}
	draft := &SKUDraft{
		Attributes: map[string]string{
			"source_sds_sku": "SKU-B",
			"Color":          "black",
			"Size":           "L",
		},
	}

	if got := MatchSubmitVariantOptionIndex(input, "", draft, 0); got != 1 {
		t.Fatalf("MatchSubmitVariantOptionIndex() = %d, want 1", got)
	}
}

func TestMatchSubmitVariantOptionIndexFallsBackByColorThenGlobalIndex(t *testing.T) {
	t.Parallel()

	input := &SubmitVariantContext{
		Variants: []SubmitVariantOption{
			{VariantSKU: "SKU-A", Color: "black", Size: "L"},
			{VariantSKU: "SKU-B", Color: "white", Size: "M"},
		},
	}

	if got := MatchSubmitVariantOptionIndex(input, "white", &SKUDraft{}, 0); got != 1 {
		t.Fatalf("color match index = %d, want 1", got)
	}
	if got := MatchSubmitVariantOptionIndex(input, "", &SKUDraft{}, 1); got != 1 {
		t.Fatalf("global fallback index = %d, want 1", got)
	}
}

func TestResolveSubmitBaseSKUAndDiscriminator(t *testing.T) {
	t.Parallel()

	input := &SubmitVariantContext{
		ProductSKU: "PRODUCT-SKU",
		Variants: []SubmitVariantOption{
			{VariantID: 101, Color: "black", Size: "L"},
		},
	}
	match := &input.Variants[0]
	draft := &SKUDraft{Attributes: map[string]string{"Color": "black", "Size": "L"}}

	if got := ResolveSubmitBaseSKU(input, draft, match, "OLD"); got != "PRODUCT-SKU" {
		t.Fatalf("ResolveSubmitBaseSKU() = %q, want PRODUCT-SKU", got)
	}
	if got := ResolveSubmitVariantDiscriminator(input, draft, match, 0, 0, "TF79B3E36"); got != "V101-TF79B3E36" {
		t.Fatalf("ResolveSubmitVariantDiscriminator() = %q, want V101-TF79B3E36", got)
	}
}

func TestInferSubmitBaseSKUFromOldTrimsVariantAndStyleSuffix(t *testing.T) {
	t.Parallel()

	if got := InferSubmitBaseSKUFromOld("MG8014186001-V101-D7E68190", "D7E68190"); got != "MG8014186001" {
		t.Fatalf("InferSubmitBaseSKUFromOld() = %q, want MG8014186001", got)
	}
	if got := InferSubmitBaseSKUFromOld("MG8014186001-D7E68190", "D7E68190"); got != "MG8014186001" {
		t.Fatalf("InferSubmitBaseSKUFromOld(style only) = %q, want MG8014186001", got)
	}
}

func TestSubmitRequiresVariantDiscriminator(t *testing.T) {
	t.Parallel()

	input := &SubmitVariantContext{
		ProductSKU: "PRODUCT-SKU",
		Variants: []SubmitVariantOption{
			{VariantSKU: "PRODUCT-SKU"},
			{VariantSKU: "PRODUCT-SKU"},
		},
	}
	if !SubmitRequiresVariantDiscriminator(input, "PRODUCT-SKU") {
		t.Fatal("SubmitRequiresVariantDiscriminator() = false, want true for duplicate base SKU")
	}
	if SubmitRequiresVariantDiscriminator(&SubmitVariantContext{VariantSKU: "SKU-1"}, "SKU-1") {
		t.Fatal("SubmitRequiresVariantDiscriminator(single variant SKU) = true, want false")
	}
}
