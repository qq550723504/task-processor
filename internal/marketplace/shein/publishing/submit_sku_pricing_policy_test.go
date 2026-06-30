package publishing

import "testing"

func TestApplyStudioSupplierSKURenamesRemapsPricingReferences(t *testing.T) {
	t.Parallel()

	state := &StudioPricingReferences{
		FinalManualPriceOverrides: map[string]float64{"OLD-SKU": 12.34},
		ManualOverrides:           map[string]float64{"OLD-SKU": 12.34},
		SKUPrices: []StudioPricingSKUReference{
			{SupplierSKU: "OLD-SKU"},
			{SupplierSKU: "OLD-SKU"},
		},
	}

	ApplyStudioSupplierSKURenames(state, []SupplierSKURename{
		{Old: "OLD-SKU", New: "NEW-SKU-1"},
		{Old: "OLD-SKU", New: "NEW-SKU-2"},
		{Old: "OLD-SKU", New: "NEW-SKU-2"},
	})

	overrides := state.FinalManualPriceOverrides
	if len(overrides) != 2 || overrides["NEW-SKU-1"] != 12.34 || overrides["NEW-SKU-2"] != 12.34 {
		t.Fatalf("final draft overrides = %#v, want fan-out to new SKUs", overrides)
	}
	if _, exists := overrides["OLD-SKU"]; exists {
		t.Fatalf("final draft overrides still contains old SKU")
	}
	if got := state.SKUPrices[0].SupplierSKU; got != "NEW-SKU-1" {
		t.Fatalf("first price SKU = %q, want NEW-SKU-1", got)
	}
	if got := state.SKUPrices[1].SupplierSKU; got != "NEW-SKU-2" {
		t.Fatalf("second price SKU = %q, want NEW-SKU-2", got)
	}
}

func TestSubmitPricingReadyRequiresPositiveSKUAndSitePrices(t *testing.T) {
	t.Parallel()

	if SubmitPricingReady(nil) {
		t.Fatal("SubmitPricingReady(nil) = true, want false")
	}
	if !SubmitPricingReady([]SubmitPricingSKUInput{{
		BasePrice:      "12.34",
		SiteBasePrices: []string{"15.99"},
	}}) {
		t.Fatal("SubmitPricingReady(valid) = false, want true")
	}
	for name, skus := range map[string][]SubmitPricingSKUInput{
		"zero base":      {{BasePrice: "0", SiteBasePrices: []string{"15.99"}}},
		"missing sites":  {{BasePrice: "12.34"}},
		"zero site":      {{BasePrice: "12.34", SiteBasePrices: []string{"0"}}},
		"non numeric":    {{BasePrice: "oops", SiteBasePrices: []string{"15.99"}}},
		"empty sku list": {},
	} {
		if SubmitPricingReady(skus) {
			t.Fatalf("SubmitPricingReady(%s) = true, want false", name)
		}
	}
}

func TestReconcileStudioPricingReferencesUsesCurrentDraftSKUAlias(t *testing.T) {
	t.Parallel()

	currentSKU := "MG8014062001-V124111-TF79B3E36-RF898D-167D3B4C"
	staleSKU := "MG8014062001-V124111-TF79B3E36-R622A0-167D3B4C"
	state := &StudioPricingReferences{
		CurrentSupplierSKUs:       []string{currentSKU},
		FinalManualPriceOverrides: map[string]float64{staleSKU: 39.99},
		ManualOverrides:           map[string]float64{staleSKU: 39.99},
		SKUPrices:                 []StudioPricingSKUReference{{SupplierSKU: staleSKU}},
	}

	if !ReconcileStudioPricingReferences(state) {
		t.Fatal("ReconcileStudioPricingReferences() = false, want true")
	}
	if got := state.SKUPrices[0].SupplierSKU; got != currentSKU {
		t.Fatalf("pricing SKU = %q, want current SKU", got)
	}
	if _, exists := state.ManualOverrides[currentSKU]; !exists {
		t.Fatalf("pricing manual overrides = %#v, want current SKU", state.ManualOverrides)
	}
	if _, exists := state.FinalManualPriceOverrides[currentSKU]; !exists {
		t.Fatalf("final draft overrides = %#v, want current SKU", state.FinalManualPriceOverrides)
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
