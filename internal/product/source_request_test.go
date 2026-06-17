package product

import "testing"

func TestSourceRequestFromFetchPreservesFields(t *testing.T) {
	req := &FetchRequest{
		TenantID:   10,
		Platform:   " SHEIN ",
		Region:     " UK ",
		ProductID:  " B001 ",
		Zipcode:    " W1A 1AA ",
		StoreID:    20,
		CategoryID: 30,
		Creator:    " tester ",
	}

	got := SourceRequestFromFetch(req)
	if got.TenantID != req.TenantID || got.Platform != "shein" || got.Region != "uk" || got.ProductID != "B001" || got.Zipcode != "W1A 1AA" || got.StoreID != req.StoreID || got.CategoryID != req.CategoryID || got.Creator != "tester" {
		t.Fatalf("SourceRequestFromFetch() = %+v, want normalized fields from %+v", got, req)
	}
}

func TestFetchRequestFromSourcePreservesFields(t *testing.T) {
	source := SourceRequestFromFetch(&FetchRequest{
		TenantID:   10,
		Platform:   " SHEIN ",
		Region:     " UK ",
		ProductID:  "B001",
		Zipcode:    "W1A 1AA",
		StoreID:    20,
		CategoryID: 30,
		Creator:    "tester",
	})

	got := FetchRequestFromSource(source)
	if got.TenantID != source.TenantID || got.Platform != source.Platform || got.Region != source.Region || got.ProductID != source.ProductID || got.Zipcode != source.Zipcode || got.StoreID != source.StoreID || got.CategoryID != source.CategoryID || got.Creator != source.Creator {
		t.Fatalf("FetchRequestFromSource() = %+v, want fields from %+v", got, source)
	}
}

func TestVariantFetchRequestPreservesSourceScopedFields(t *testing.T) {
	base := &FetchRequest{
		TenantID:   10,
		Platform:   " SHEIN ",
		Region:     " UK ",
		ProductID:  "parent",
		Zipcode:    " W1A 1AA ",
		StoreID:    20,
		CategoryID: 30,
		Creator:    " tester ",
	}

	got := VariantFetchRequest(base, " B002 ")
	if got.ProductID != "B002" {
		t.Fatalf("ProductID = %q, want variant ASIN", got.ProductID)
	}
	if got.TenantID != 10 || got.Platform != "shein" || got.Region != "uk" || got.Zipcode != "W1A 1AA" || got.StoreID != 20 || got.CategoryID != 30 || got.Creator != "tester" {
		t.Fatalf("VariantFetchRequest() = %+v, want normalized source-scoped fields", got)
	}
}
