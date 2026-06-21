package shein

import (
	"testing"

	common "task-processor/internal/publishing/common"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestNormalizeStudioSubmitSupplierSKUsUpdatesPayloadsAndPricing(t *testing.T) {
	t.Parallel()

	oldSKU := "MG8014186001-D7E68190"
	pkg := &Package{
		DraftPayload: &RequestDraft{
			SKCList: []SKCRequestDraft{
				{
					SaleAttribute: &ResolvedSaleAttribute{Value: "black"},
					SKUList: []SKUDraft{{
						SupplierSKU: oldSKU,
						Attributes: map[string]string{
							"Color": "black",
							"Size":  "one size",
						},
					}},
				},
				{
					SaleAttribute: &ResolvedSaleAttribute{Value: "white"},
					SKUList: []SKUDraft{{
						SupplierSKU: oldSKU,
						Attributes: map[string]string{
							"Color": "white",
							"Size":  "one size",
						},
					}},
				},
			},
		},
		SkcList: []SKCPackage{
			{SKUs: []common.Variant{{SKU: oldSKU}}},
			{SKUs: []common.Variant{{SKU: oldSKU}}},
		},
		PreviewPayload: &sheinproduct.Product{
			SKCList: []sheinproduct.SKC{
				{SKUS: []sheinproduct.SKU{{SupplierSKU: oldSKU}}},
				{SKUS: []sheinproduct.SKU{{SupplierSKU: oldSKU}}},
			},
		},
		FinalSubmissionDraft: &FinalDraft{
			ManualPriceOverrides: map[string]float64{oldSKU: 25.55},
		},
		Pricing: &PricingReview{
			ManualOverrides: map[string]float64{oldSKU: 25.55},
			SKUPrices: []SKUPriceReview{
				{SupplierSKU: oldSKU},
				{SupplierSKU: oldSKU},
			},
		},
	}

	changed := NormalizeStudioSubmitSupplierSKUs(pkg, StudioSubmitSKUContext{
		StyleID:           "D7E68190",
		TaskDiscriminator: "TSUBMITTA",
		Variant: &SubmitVariantContext{
			ProductSKU: "MG8014186001",
			StyleID:    "D7E68190",
			Variants: []SubmitVariantOption{
				{VariantID: 101, Color: "black", Size: "one size"},
				{VariantID: 102, Color: "white", Size: "one size"},
			},
		},
	})

	if !changed {
		t.Fatal("NormalizeStudioSubmitSupplierSKUs() = false, want true")
	}
	want := []string{
		"MG8014186001-V101-TSUBMITTA-D7E68190",
		"MG8014186001-V102-TSUBMITTA-D7E68190",
	}
	for i := range want {
		if got := pkg.DraftPayload.SKCList[i].SKUList[0].SupplierSKU; got != want[i] {
			t.Fatalf("draft sku[%d] = %q, want %q", i, got, want[i])
		}
		if got := pkg.SkcList[i].SKUs[0].SKU; got != want[i] {
			t.Fatalf("package sku[%d] = %q, want %q", i, got, want[i])
		}
		if got := pkg.PreviewPayload.SKCList[i].SKUS[0].SupplierSKU; got != want[i] {
			t.Fatalf("preview sku[%d] = %q, want %q", i, got, want[i])
		}
		if got := pkg.Pricing.SKUPrices[i].SupplierSKU; got != want[i] {
			t.Fatalf("price sku[%d] = %q, want %q", i, got, want[i])
		}
	}
	if len(pkg.FinalSubmissionDraft.ManualPriceOverrides) != 2 ||
		pkg.FinalSubmissionDraft.ManualPriceOverrides[want[0]] != 25.55 ||
		pkg.FinalSubmissionDraft.ManualPriceOverrides[want[1]] != 25.55 {
		t.Fatalf("final draft overrides = %#v, want fan-out to normalized SKUs", pkg.FinalSubmissionDraft.ManualPriceOverrides)
	}
}

func TestNormalizeStudioSubmitSupplierSKUsReconcilesOnlyPricingWhenSKUsUnchanged(t *testing.T) {
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

	changed := NormalizeStudioSubmitSupplierSKUs(pkg, StudioSubmitSKUContext{
		StyleID:           "167D3B4C",
		TaskDiscriminator: "TF79B3E36-RF898D",
		Variant: &SubmitVariantContext{
			ProductSKU: "MG8014062001",
			StyleID:    "167D3B4C",
			Variants: []SubmitVariantOption{{
				VariantID:  124111,
				VariantSKU: "MG8014062001",
			}},
		},
	})

	if !changed {
		t.Fatal("NormalizeStudioSubmitSupplierSKUs() = false, want pricing reconciliation")
	}
	if got := pkg.Pricing.SKUPrices[0].SupplierSKU; got != currentSKU {
		t.Fatalf("pricing sku = %q, want %q", got, currentSKU)
	}
	if _, exists := pkg.FinalSubmissionDraft.ManualPriceOverrides[currentSKU]; !exists {
		t.Fatalf("final draft overrides = %#v, want current SKU", pkg.FinalSubmissionDraft.ManualPriceOverrides)
	}
}
