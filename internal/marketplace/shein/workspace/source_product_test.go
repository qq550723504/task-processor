package workspace

import (
	"testing"

	"task-processor/internal/product/catalog/canonical"
)

func TestBuildSourceProductSummary(t *testing.T) {
	t.Parallel()

	product := &canonical.Product{
		Title:        "Bottle",
		CategoryPath: []string{"Home", "Kitchen"},
		Specifications: &canonical.ProductSpecs{Technical: map[string]string{
			"parent_product_id": "238915",
			"variant_id":        "238916",
		}},
		Attributes: map[string]canonical.Attribute{
			"sku":   {Value: "SKU-1"},
			"brand": {Value: "Acme"},
		},
	}

	summary := BuildSourceProductSummary(product)
	if summary == nil {
		t.Fatal("expected summary")
	}
	if summary.Title != "Bottle" || summary.SKU != "SKU-1" {
		t.Fatalf("summary = %+v", summary)
	}
	if summary.ParentProductID != "" || summary.VariantID != "" {
		t.Fatalf("source ids = parent %q variant %q, want empty without explicit SDS provenance", summary.ParentProductID, summary.VariantID)
	}
	if summary.Attributes["brand"] != "Acme" {
		t.Fatalf("attributes = %+v", summary.Attributes)
	}
}
