package sourcing

import "testing"

func TestVariantSourceRequestPreservesSourceScopedFields(t *testing.T) {
	base := SourceRequest{
		TenantID:   10,
		Platform:   "shein",
		Region:     "uk",
		ProductID:  "parent",
		Zipcode:    "W1A 1AA",
		StoreID:    20,
		CategoryID: 30,
		Creator:    "tester",
	}

	got := VariantSourceRequest(base, "variant")
	if got.ProductID != "variant" {
		t.Fatalf("ProductID = %q, want variant", got.ProductID)
	}
	if got.Zipcode != "W1A 1AA" {
		t.Fatalf("Zipcode = %q, want inherited zipcode", got.Zipcode)
	}
	if got.Platform != base.Platform || got.Region != base.Region || got.TenantID != base.TenantID || got.StoreID != base.StoreID || got.CategoryID != base.CategoryID || got.Creator != base.Creator {
		t.Fatalf("source-scoped fields were not preserved: %+v", got)
	}
}
