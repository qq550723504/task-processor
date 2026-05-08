package common

import (
	"testing"

	"task-processor/internal/catalog/canonical"
)

func TestBuildVariantsFallsBackToColorAndSizeMatrix(t *testing.T) {
	product := &canonical.Product{
		Attributes: map[string]canonical.Attribute{
			"color": {Value: "Red, Blue"},
			"size":  {Value: "42/43"},
			"price": {Value: "19.9"},
		},
		Images: []canonical.Image{{URL: "main.jpg"}},
	}

	variants := BuildVariants(product)
	if len(variants) != 4 {
		t.Fatalf("variant count = %d, want 4", len(variants))
	}
	if variants[0].SKU != "DEFAULT-001-RED-42" {
		t.Fatalf("first sku = %q, want DEFAULT-001-RED-42", variants[0].SKU)
	}
	if variants[0].Attributes["color"] != "Red" || variants[0].Attributes["size"] != "42" {
		t.Fatalf("first attributes = %+v", variants[0].Attributes)
	}
	if !variants[0].IsDefault {
		t.Fatalf("expected first fallback variant to be default")
	}
	if variants[3].Attributes["color"] != "Blue" || variants[3].Attributes["size"] != "43" {
		t.Fatalf("last attributes = %+v", variants[3].Attributes)
	}
}

func TestBuildVariantsFallsBackToSingleDefaultVariant(t *testing.T) {
	product := &canonical.Product{
		Attributes: map[string]canonical.Attribute{
			"material": {Value: "Cotton"},
		},
	}

	variants := BuildVariants(product)
	if len(variants) != 1 {
		t.Fatalf("variant count = %d, want 1", len(variants))
	}
	if variants[0].SKU != "DEFAULT-001" {
		t.Fatalf("sku = %q, want DEFAULT-001", variants[0].SKU)
	}
	if variants[0].Attributes["material"] != "Cotton" {
		t.Fatalf("attributes = %+v", variants[0].Attributes)
	}
}
