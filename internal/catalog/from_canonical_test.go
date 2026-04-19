package catalog

import (
	"testing"

	"task-processor/internal/productenrich"
)

func TestBuildProductBuildsCatalogSnapshot(t *testing.T) {
	t.Parallel()

	product := BuildProduct(&productenrich.CanonicalProduct{
		Title:         "Wireless Earbuds",
		Brand:         "Acme",
		CategoryPath:  []string{"Electronics", "Audio"},
		Description:   "ANC earbuds",
		SellingPoints: []string{"ANC", "Bluetooth"},
		SEOKeywords:   []string{"earbuds"},
		FieldTraces: map[string]productenrich.FieldTrace{
			"title": {
				Sources: []productenrich.CanonicalSource{{Type: productenrich.CanonicalSourceUserText, Detail: "user text"}},
			},
		},
		Attributes: map[string]productenrich.CanonicalAttribute{
			"color": {
				Value: "Black",
				Trace: productenrich.FieldTrace{
					NeedsReview: true,
					Sources: []productenrich.CanonicalSource{{
						Type:   productenrich.CanonicalSourceUserImage,
						Detail: "uploaded image",
					}},
				},
			},
		},
		Images: []productenrich.CanonicalImage{{
			URL:  "https://example.com/1.jpg",
			Role: "primary",
		}},
		Variants: []productenrich.CanonicalVariant{{
			SKU: "SKU-1",
			Attributes: map[string]productenrich.CanonicalAttribute{
				"size": {Value: "M"},
			},
			Price: &productenrich.PriceInfo{
				Currency: "USD",
				Amount:   29.9,
			},
		}},
	})

	if product == nil {
		t.Fatal("expected product")
	}
	if product.Title != "Wireless Earbuds" {
		t.Fatalf("title = %q", product.Title)
	}
	if len(product.Attributes) != 1 || product.Attributes[0].Name != "color" {
		t.Fatalf("attributes = %+v", product.Attributes)
	}
	if len(product.Variants) != 1 || product.Variants[0].SKU != "SKU-1" {
		t.Fatalf("variants = %+v", product.Variants)
	}
	if product.Review == nil || !product.Review.NeedsReview {
		t.Fatalf("review = %+v, want needs review", product.Review)
	}
	if len(product.Sources) == 0 {
		t.Fatalf("sources = %+v, want collected sources", product.Sources)
	}
}
