package catalog

import (
	"testing"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/productenrich"
)

func TestBuildProductBuildsCatalogSnapshot(t *testing.T) {
	t.Parallel()

	product := BuildProduct(&canonical.Product{
		Title:         "Wireless Earbuds",
		Brand:         "Acme",
		CategoryPath:  []string{"Electronics", "Audio"},
		Description:   "ANC earbuds",
		SellingPoints: []string{"ANC", "Bluetooth"},
		SEOKeywords:   []string{"earbuds"},
		FieldTraces: map[string]canonical.FieldTrace{
			"title": {
				Sources: []canonical.Source{{Type: canonical.SourceUserText, Detail: "user text"}},
			},
		},
		Attributes: map[string]canonical.Attribute{
			"color": {
				Value: "Black",
				Trace: canonical.FieldTrace{
					NeedsReview: true,
					Sources: []canonical.Source{{
						Type:   canonical.SourceUserImage,
						Detail: "uploaded image",
					}},
				},
			},
		},
		Images: []canonical.Image{{
			URL:  "https://example.com/1.jpg",
			Role: "primary",
		}},
		Variants: []canonical.Variant{{
			SKU: "SKU-1",
			Attributes: map[string]canonical.Attribute{
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
