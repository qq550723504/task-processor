package listingkit

import (
	"testing"

	"task-processor/internal/productenrich"
)

func TestBuildPlatformVariantsFallsBackToDefaultVariant(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		Attributes: map[string]productenrich.CanonicalAttribute{
			"color": {Value: "Black"},
		},
		Images: []productenrich.CanonicalImage{{URL: "main.jpg"}},
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
