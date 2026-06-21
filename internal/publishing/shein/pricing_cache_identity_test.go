package shein

import "testing"

func TestPricingCacheKeyIgnoresSubmitTokensButTracksCost(t *testing.T) {
	t.Parallel()

	req := &BuildRequest{SheinStoreID: 870}
	first := pricingCachePackageWithSKU("MG8006062004-V96697-TB9B6431C-RABC12", "12.345")
	second := pricingCachePackageWithSKU("MG8006062004-V11111-TCDEF1234-RDEF34", "12.345")
	changedCost := pricingCachePackageWithSKU("MG8006062004-V11111-TCDEF1234-RDEF34", "13.00")

	firstKey := PricingCacheKey(req, first, PricingRule{})
	secondKey := PricingCacheKey(req, second, PricingRule{})
	changedCostKey := PricingCacheKey(req, changedCost, PricingRule{})

	if firstKey == "" {
		t.Fatal("PricingCacheKey() returned empty key")
	}
	if firstKey != secondKey {
		t.Fatalf("PricingCacheKey() changed for token-only SKU differences: %q vs %q", firstKey, secondKey)
	}
	if firstKey == changedCostKey {
		t.Fatalf("PricingCacheKey() did not change after cost changed: %q", firstKey)
	}
}

func TestPricingReviewApplicableRequiresCurrentCostAndFinalPrice(t *testing.T) {
	t.Parallel()

	pkg := pricingCachePackageWithSKU("MG8006062004-V96697-TB9B6431C-RABC12", "12.345")
	review := &PricingReview{
		Ready: true,
		SKUPrices: []SKUPriceReview{{
			SupplierSKU: "MG8006062004-V12345-TAAAA1111-RBBBB2",
			CostCNY:     12.35,
			FinalPrice:  19.99,
		}},
	}

	if !PricingReviewApplicable(pkg, review) {
		t.Fatal("PricingReviewApplicable() = false, want true")
	}
	review.SKUPrices[0].CostCNY = 12.36
	if PricingReviewApplicable(pkg, review) {
		t.Fatal("PricingReviewApplicable() = true after cost mismatch, want false")
	}
	review.SKUPrices[0].CostCNY = 12.35
	review.SKUPrices[0].FinalPrice = 0
	if PricingReviewApplicable(pkg, review) {
		t.Fatal("PricingReviewApplicable() = true with empty final price, want false")
	}
}

func TestNormalizePublishedPricingReviewClonesAndTrims(t *testing.T) {
	t.Parallel()

	pkg := pricingCachePackageWithSKU("MG8006062004-V96697-TB9B6431C-RABC12", "12.345")
	pkg.Pricing = &PricingReview{
		Ready: true,
		SKUPrices: []SKUPriceReview{{
			SupplierSKU:  "  MG8006062004-V12345-TAAAA1111-RBBBB2  ",
			SupplierCode: "  MG8006062004  ",
			FinalPrice:   19.99,
		}},
	}

	got := NormalizePublishedPricingReview(pkg)

	if got == nil {
		t.Fatal("NormalizePublishedPricingReview() returned nil")
	}
	if got == pkg.Pricing {
		t.Fatal("NormalizePublishedPricingReview() returned original pointer")
	}
	if got.SKUPrices[0].SupplierSKU != "MG8006062004-V12345-TAAAA1111-RBBBB2" {
		t.Fatalf("supplier SKU = %q, want trimmed value", got.SKUPrices[0].SupplierSKU)
	}
	if got.SKUPrices[0].SupplierCode != "MG8006062004" {
		t.Fatalf("supplier code = %q, want trimmed value", got.SKUPrices[0].SupplierCode)
	}
	if pkg.Pricing.SKUPrices[0].SupplierSKU == got.SKUPrices[0].SupplierSKU {
		t.Fatal("NormalizePublishedPricingReview() mutated source review")
	}
}

func pricingCachePackageWithSKU(supplierSKU string, costPrice string) *Package {
	return &Package{
		CategoryID:     123,
		CategoryIDList: []int{1, 12, 123},
		CategoryPath:   []string{" Women ", " Dresses "},
		SpuName:        "Summer Dress",
		DraftPayload: &RequestDraft{
			SKCList: []SKCRequestDraft{{
				SupplierCode: "MG8006062004",
				SKUList: []SKUDraft{{
					SupplierSKU: supplierSKU,
					Attributes:  map[string]string{"source_sds_sku": "MG8006062004"},
					CostPrice:   costPrice,
					Currency:    "CNY",
				}},
			}},
		},
	}
}
