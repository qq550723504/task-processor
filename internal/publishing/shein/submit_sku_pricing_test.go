package shein

import "testing"

func TestApplyStudioSupplierSKURenamesRemapsPricingReferences(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		FinalSubmissionDraft: &FinalDraft{
			ManualPriceOverrides: map[string]float64{"OLD-SKU": 12.34},
		},
		Pricing: &PricingReview{
			ManualOverrides: map[string]float64{"OLD-SKU": 12.34},
			SKUPrices: []SKUPriceReview{
				{SupplierSKU: "OLD-SKU"},
				{SupplierSKU: "OLD-SKU"},
			},
		},
	}

	ApplyStudioSupplierSKURenames(pkg, []SupplierSKURename{
		{Old: "OLD-SKU", New: "NEW-SKU-1"},
		{Old: "OLD-SKU", New: "NEW-SKU-2"},
		{Old: "OLD-SKU", New: "NEW-SKU-2"},
	})

	overrides := pkg.FinalSubmissionDraft.ManualPriceOverrides
	if len(overrides) != 2 || overrides["NEW-SKU-1"] != 12.34 || overrides["NEW-SKU-2"] != 12.34 {
		t.Fatalf("final draft overrides = %#v, want fan-out to new SKUs", overrides)
	}
	if _, exists := overrides["OLD-SKU"]; exists {
		t.Fatalf("final draft overrides still contains old SKU")
	}
	if got := pkg.Pricing.SKUPrices[0].SupplierSKU; got != "NEW-SKU-1" {
		t.Fatalf("first price SKU = %q, want NEW-SKU-1", got)
	}
	if got := pkg.Pricing.SKUPrices[1].SupplierSKU; got != "NEW-SKU-2" {
		t.Fatalf("second price SKU = %q, want NEW-SKU-2", got)
	}
}

func TestReconcileStudioPricingReferencesUsesCurrentDraftSKUAlias(t *testing.T) {
	t.Parallel()

	currentSKU := "MG8014062001-V124111-TF79B3E36-RF898D-167D3B4C"
	staleSKU := "MG8014062001-V124111-TF79B3E36-R622A0-167D3B4C"
	pkg := &Package{
		DraftPayload: &RequestDraft{
			SKCList: []SKCRequestDraft{{
				SKUList: []SKUDraft{{SupplierSKU: currentSKU}},
			}},
		},
		FinalSubmissionDraft: &FinalDraft{
			ManualPriceOverrides: map[string]float64{staleSKU: 39.99},
		},
		Pricing: &PricingReview{
			ManualOverrides: map[string]float64{staleSKU: 39.99},
			SKUPrices:       []SKUPriceReview{{SupplierSKU: staleSKU}},
		},
	}

	if !ReconcileStudioPricingReferences(pkg) {
		t.Fatal("ReconcileStudioPricingReferences() = false, want true")
	}
	if got := pkg.Pricing.SKUPrices[0].SupplierSKU; got != currentSKU {
		t.Fatalf("pricing SKU = %q, want current SKU", got)
	}
	if _, exists := pkg.Pricing.ManualOverrides[currentSKU]; !exists {
		t.Fatalf("pricing manual overrides = %#v, want current SKU", pkg.Pricing.ManualOverrides)
	}
	if _, exists := pkg.FinalSubmissionDraft.ManualPriceOverrides[currentSKU]; !exists {
		t.Fatalf("final draft overrides = %#v, want current SKU", pkg.FinalSubmissionDraft.ManualPriceOverrides)
	}
}

func TestStudioPricingSKUAliasDropsTaskRequestAndStyleTokens(t *testing.T) {
	t.Parallel()

	got := StudioPricingSKUAlias("MG8014062001-V124111-TF79B3E36-RF898D-167D3B4C")
	want := "MG8014062001-V124111"
	if got != want {
		t.Fatalf("StudioPricingSKUAlias() = %q, want %q", got, want)
	}
}
