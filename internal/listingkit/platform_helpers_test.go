package listingkit

import (
	"testing"

	"task-processor/internal/catalog/canonical"
)

func TestBuildPlatformVariantsFallsBackToDefaultVariant(t *testing.T) {
	canonical := &canonical.Product{
		Attributes: map[string]canonical.Attribute{
			"color": {Value: "Black"},
		},
		Images: []canonical.Image{{URL: "main.jpg"}},
	}

	variants := buildPlatformVariants(canonical)
	if len(variants) != 1 {
		t.Fatalf("variant count = %d, want 1", len(variants))
	}
	if variants[0].SKU != "DEFAULT-001" {
		t.Fatalf("default sku = %q, want DEFAULT-001", variants[0].SKU)
	}
	if !variants[0].IsDefault {
		t.Fatalf("expected fallback variant to be default")
	}
	if variants[0].Image != "main.jpg" {
		t.Fatalf("fallback image = %q, want main.jpg", variants[0].Image)
	}
}
