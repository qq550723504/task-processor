package listingkit

import (
	"testing"

	"task-processor/internal/productenrich"
)

func TestStudioVariantsAddsVariantDiscriminatorWhenVariantSKUMissing(t *testing.T) {
	sds := &SDSSyncOptions{
		ProductSKU: "MG8014186001",
		StyleID:    "d7e6-8190-abcdef",
		Variants: []SDSSyncVariantOption{
			{VariantID: 101, Color: "黑色", Size: "均码"},
			{VariantID: 102, Color: "白色", Size: "均码"},
		},
	}

	variants := studioVariants(sds, nil, productenrich.FieldTrace{})
	if len(variants) != 2 {
		t.Fatalf("variant count = %d, want 2", len(variants))
	}
	if variants[0].SKU != "MG8014186001-V101-D7E68190" {
		t.Fatalf("first sku = %q", variants[0].SKU)
	}
	if variants[1].SKU != "MG8014186001-V102-D7E68190" {
		t.Fatalf("second sku = %q", variants[1].SKU)
	}
	if variants[0].SKU == variants[1].SKU {
		t.Fatalf("expected unique skus, got %q", variants[0].SKU)
	}
}

func TestStudioVariantsDeduplicatesRepeatedBaseSKU(t *testing.T) {
	sds := &SDSSyncOptions{
		ProductSKU: "MG8014186001",
		StyleID:    "d7e6-8190-abcdef",
		Variants: []SDSSyncVariantOption{
			{VariantSKU: "MG8014186001", Color: "Black", Size: "One Size"},
			{VariantSKU: "MG8014186001", Color: "Gray", Size: "One Size"},
		},
	}

	variants := studioVariants(sds, nil, productenrich.FieldTrace{})
	if len(variants) != 2 {
		t.Fatalf("variant count = %d, want 2", len(variants))
	}
	if variants[0].SKU != "MG8014186001-BLACK-ONE-SIZE-V1-D7E68190" {
		t.Fatalf("first sku = %q", variants[0].SKU)
	}
	if variants[1].SKU != "MG8014186001-GRAY-ONE-SIZE-V2-D7E68190" {
		t.Fatalf("second sku = %q", variants[1].SKU)
	}
	if variants[0].SKU == variants[1].SKU {
		t.Fatalf("expected unique skus, got %q", variants[0].SKU)
	}
}

func TestStudioVariantsPreservesDistinctVariantSKUs(t *testing.T) {
	sds := &SDSSyncOptions{
		ProductSKU: "MG8014186001",
		StyleID:    "d7e6-8190-abcdef",
		Variants: []SDSSyncVariantOption{
			{VariantSKU: "MG8014186001-BLK", Color: "Black", Size: "One Size"},
			{VariantSKU: "MG8014186001-WHT", Color: "White", Size: "One Size"},
		},
	}

	variants := studioVariants(sds, nil, productenrich.FieldTrace{})
	if len(variants) != 2 {
		t.Fatalf("variant count = %d, want 2", len(variants))
	}
	if variants[0].SKU != "MG8014186001-BLK-D7E68190" {
		t.Fatalf("first sku = %q", variants[0].SKU)
	}
	if variants[1].SKU != "MG8014186001-WHT-D7E68190" {
		t.Fatalf("second sku = %q", variants[1].SKU)
	}
}
