package workspace

import (
	"testing"

	"task-processor/internal/catalog/canonical"
)

func TestBuildSourceProductSummary(t *testing.T) {
	t.Parallel()

	product := &canonical.Product{
		Title:        "Bottle",
		CategoryPath: []string{"Home", "Kitchen"},
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
	if summary.Attributes["brand"] != "Acme" {
		t.Fatalf("attributes = %+v", summary.Attributes)
	}
}
