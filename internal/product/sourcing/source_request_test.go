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

func TestNormalizeSourceRequestTrimsAndNormalizesIdentityFields(t *testing.T) {
	got := NormalizeSourceRequest(SourceRequest{
		Platform:  " Amazon ",
		Region:    " UK ",
		ProductID: " B001 ",
		Zipcode:   " W1A 1AA ",
		Creator:   " tester ",
	})

	if got.Platform != "amazon" || got.Region != "uk" || got.ProductID != "B001" || got.Zipcode != "W1A 1AA" || got.Creator != "tester" {
		t.Fatalf("NormalizeSourceRequest() = %+v, want normalized source fields", got)
	}
}

func TestSourceRequestIdentityKey(t *testing.T) {
	identity := SourceRequest{
		Platform:  " Amazon ",
		Region:    " UK ",
		ProductID: " B001 ",
		StoreID:   42,
	}.Identity()

	if identity.Platform != "amazon" || identity.Region != "uk" || identity.ProductID != "B001" || identity.StoreID != 42 {
		t.Fatalf("Identity() = %+v, want normalized identity", identity)
	}
	if got := identity.Key(); got != "amazon:uk:B001:42" {
		t.Fatalf("Key() = %q, want amazon:uk:B001:42", got)
	}
}
