package listingkit

import (
	"testing"

	"task-processor/internal/catalog/canonical"
)

func TestBuildSheinSourceProductSummary(t *testing.T) {
	t.Parallel()

	product := &canonical.Product{
		Title:        "Bottle",
		CategoryPath: []string{"Home", "Kitchen"},
		Attributes: map[string]canonical.Attribute{
			"sku":   {Value: "SKU-1"},
			"empty": {Value: ""},
		},
		Images: []canonical.Image{{URL: "https://cdn.example.com/main.jpg"}},
		Variants: []canonical.Variant{{
			SKU: "SKU-1A",
			Attributes: map[string]canonical.Attribute{
				"Size":  {Value: "One Size"},
				"Color": {Value: "Black"},
			},
			Images: []canonical.Image{{URL: "https://cdn.example.com/variant.jpg"}},
		}},
	}

	summary := buildSheinSourceProductSummary(product)
	if summary == nil {
		t.Fatal("expected summary")
	}
	if summary.Title != "Bottle" || summary.SKU != "SKU-1" {
		t.Fatalf("summary = %+v", summary)
	}
	if summary.VariantSKU != "SKU-1A" || summary.VariantSize != "One Size" || summary.VariantColor != "Black" {
		t.Fatalf("variant summary = %+v", summary)
	}
	if len(summary.ImageURLs) != 2 {
		t.Fatalf("image urls = %+v", summary.ImageURLs)
	}
}
