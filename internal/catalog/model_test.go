package catalog

import (
	"encoding/json"
	"testing"
)

func TestProductUnmarshalAcceptsKeyedAttributes(t *testing.T) {
	t.Parallel()

	var product Product
	err := json.Unmarshal([]byte(`{
		"title": "Legacy product",
		"attributes": {
			"product_size": {
				"value": "S",
				"trace": {
					"sources": [{"type": "derived", "detail": "legacy canonical snapshot"}],
					"confidence": 0.99
				}
			},
			"material": {"value": "Polyester"}
		}
	}`), &product)
	if err != nil {
		t.Fatalf("unmarshal product: %v", err)
	}
	if len(product.Attributes) != 2 {
		t.Fatalf("attributes = %+v, want 2 items", product.Attributes)
	}
	if product.Attributes[0].Name != "material" || product.Attributes[0].Value != "Polyester" {
		t.Fatalf("first attribute = %+v, want keyed material", product.Attributes[0])
	}
	if product.Attributes[1].Name != "product_size" || product.Attributes[1].Value != "S" {
		t.Fatalf("second attribute = %+v, want keyed product_size", product.Attributes[1])
	}
	if got := product.Attributes[1].Trace.Sources[0].Detail; got != "legacy canonical snapshot" {
		t.Fatalf("trace detail = %q", got)
	}
}

func TestVariantUnmarshalAcceptsKeyedAttributes(t *testing.T) {
	t.Parallel()

	var variant Variant
	err := json.Unmarshal([]byte(`{
		"sku": "SKU-1",
		"attributes": {
			"Size": {"value": "XL"}
		}
	}`), &variant)
	if err != nil {
		t.Fatalf("unmarshal variant: %v", err)
	}
	if len(variant.Attributes) != 1 || variant.Attributes[0].Name != "Size" || variant.Attributes[0].Value != "XL" {
		t.Fatalf("variant attributes = %+v, want keyed Size", variant.Attributes)
	}
}
