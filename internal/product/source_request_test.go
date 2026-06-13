package product

import "testing"

func TestSourceRequestFromFetchPreservesFields(t *testing.T) {
	req := &FetchRequest{
		TenantID:   10,
		Platform:   "shein",
		Region:     "uk",
		ProductID:  "B001",
		Zipcode:    "W1A 1AA",
		StoreID:    20,
		CategoryID: 30,
		Creator:    "tester",
	}

	got := SourceRequestFromFetch(req)
	if got.TenantID != req.TenantID || got.Platform != req.Platform || got.Region != req.Region || got.ProductID != req.ProductID || got.Zipcode != req.Zipcode || got.StoreID != req.StoreID || got.CategoryID != req.CategoryID || got.Creator != req.Creator {
		t.Fatalf("SourceRequestFromFetch() = %+v, want fields from %+v", got, req)
	}
}

func TestFetchRequestFromSourcePreservesFields(t *testing.T) {
	source := SourceRequestFromFetch(&FetchRequest{
		TenantID:   10,
		Platform:   "shein",
		Region:     "uk",
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
