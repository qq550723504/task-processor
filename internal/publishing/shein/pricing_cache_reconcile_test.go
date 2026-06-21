package shein

import "testing"

func TestReconcilePricingCacheReviewRemapsManualOverrideToCurrentDraftSKU(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		RequestDraft: &RequestDraft{
			SKCList: []SKCRequestDraft{{
				SupplierCode: "MG8006062004-EF926739",
				SKUList: []SKUDraft{{
					SupplierSKU: "MG8006062004-EF926739",
					Attributes:  map[string]string{"source_sds_sku": "MG8006062004"},
				}},
			}},
		},
	}
	review := &PricingReview{
		SKUPrices: []SKUPriceReview{{
			SupplierSKU:  "MG8006062004-V96697-TB9B6431C-RA8CD5E-E5BADD24",
			SupplierCode: "MG8006062004-9D6E1EC4",
			FinalPrice:   143,
		}},
		ManualOverrides: map[string]float64{
			"MG8006062004-V96697-TAB634DC8-R14A9AC-92B5A7B8": 82,
		},
		Ready: true,
	}

	got := ReconcilePricingCacheReview(pkg, review)

	if got != review {
		t.Fatal("ReconcilePricingCacheReview returned a different review pointer")
	}
	if got.SKUPrices[0].SupplierSKU != "MG8006062004-EF926739" {
		t.Fatalf("supplier SKU = %q, want current draft SKU", got.SKUPrices[0].SupplierSKU)
	}
	if got.SKUPrices[0].SupplierCode != "MG8006062004-EF926739" {
		t.Fatalf("supplier code = %q, want current supplier code", got.SKUPrices[0].SupplierCode)
	}
	if got.SKUPrices[0].FinalPrice != 82 || !got.SKUPrices[0].Manual {
		t.Fatalf("price = %+v, want remapped manual price 82", got.SKUPrices[0])
	}
	if got.ManualOverrides["MG8006062004-EF926739"] != 82 {
		t.Fatalf("manual overrides = %#v, want current SKU override", got.ManualOverrides)
	}
	if _, exists := got.ManualOverrides["MG8006062004-V96697-TAB634DC8-R14A9AC-92B5A7B8"]; exists {
		t.Fatalf("manual overrides retained stale SKU: %#v", got.ManualOverrides)
	}
}
