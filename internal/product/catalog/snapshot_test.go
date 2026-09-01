package catalog

import (
	"encoding/json"
	"testing"
)

func TestProductSnapshotUnmarshalAcceptsKeyedAttributes(t *testing.T) {
	t.Parallel()

	var snapshot ProductSnapshot
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
	}`), &snapshot)
	if err != nil {
		t.Fatalf("unmarshal product snapshot: %v", err)
	}
	if len(snapshot.Attributes) != 2 {
		t.Fatalf("attributes = %+v, want 2 items", snapshot.Attributes)
	}
	if snapshot.Attributes[0].Name != "material" || snapshot.Attributes[0].Value != "Polyester" {
		t.Fatalf("first attribute = %+v, want keyed material", snapshot.Attributes[0])
	}
	if snapshot.Attributes[1].Name != "product_size" || snapshot.Attributes[1].Value != "S" {
		t.Fatalf("second attribute = %+v, want keyed product_size", snapshot.Attributes[1])
	}
	if got := snapshot.Attributes[1].Trace.Sources[0].Detail; got != "legacy canonical snapshot" {
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
