package listingkit

import (
	"testing"

	"task-processor/internal/productenrich"
)

func TestStudioAttributesAndSpecificationsIncludeRichSDSFields(t *testing.T) {
	sds := &SDSSyncOptions{
		ProductSKU:             "MG17701062",
		Material:               "复合板",
		MaterialDescription:    "优选复合板材质",
		ProductionProcess:      "UV打印",
		ProductPerformance:     "静音无声，轻奢质地。",
		ApplicableScenarios:    "办公室、卧室、客厅",
		SpecialDescription:     "挂钟不含电池。",
		ProductSize:            "25*25cm",
		PackagingSpecification: "30*30*5cm，0.45kg",
		VariantSize:            "25cm/9.8inch",
		VariantColor:           "White",
	}

	attrs := studioAttributes(sds, productenrich.FieldTrace{})
	if attrs["material_description"].Value != "优选复合板材质" {
		t.Fatalf("material_description = %+v", attrs["material_description"])
	}
	if attrs["product_performance"].Value == "" {
		t.Fatalf("product_performance = %+v", attrs["product_performance"])
	}
	if attrs["product_size"].Value != "25*25cm" {
		t.Fatalf("product_size = %+v", attrs["product_size"])
	}
	if attrs["packaging_specification"].Value == "" {
		t.Fatalf("packaging_specification = %+v", attrs["packaging_specification"])
	}

	specs := studioSpecifications(sds)
	if specs == nil {
		t.Fatal("specs = nil")
	}
	if specs.Technical["product_size"] != "25*25cm" {
		t.Fatalf("technical product_size = %+v", specs.Technical)
	}
	if specs.Technical["packaging_specification"] != "30*30*5cm，0.45kg" {
		t.Fatalf("technical packaging_specification = %+v", specs.Technical)
	}
	if specs.Technical["product_performance"] == "" {
		t.Fatalf("technical product_performance = %+v", specs.Technical)
	}
}

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

func TestStudioVariantsPreserveVariantDimensionsAndWeight(t *testing.T) {
	sds := &SDSSyncOptions{
		ProductSKU: "MG8014186001",
		StyleID:    "d7e6-8190-abcdef",
		Variants: []SDSSyncVariantOption{
			{VariantID: 101, Color: "Black", Size: "40x60cm", Weight: 120, BoxLength: 40, BoxWidth: 30, BoxHeight: 2},
			{VariantID: 102, Color: "Black", Size: "50x80cm", Weight: 180, BoxLength: 50, BoxWidth: 40, BoxHeight: 3},
		},
	}

	variants := studioVariants(sds, nil, productenrich.FieldTrace{})
	if len(variants) != 2 {
		t.Fatalf("variant count = %d, want 2", len(variants))
	}
	if variants[0].Dimensions == nil || variants[1].Dimensions == nil {
		t.Fatalf("expected variant dimensions to be preserved: %+v", variants)
	}
	if variants[0].Dimensions.Length != 40 || variants[0].Dimensions.Width != 30 || variants[0].Dimensions.Height != 2 {
		t.Fatalf("first dimensions = %+v", variants[0].Dimensions)
	}
	if variants[1].Dimensions.Length != 50 || variants[1].Dimensions.Width != 40 || variants[1].Dimensions.Height != 3 {
		t.Fatalf("second dimensions = %+v", variants[1].Dimensions)
	}
	if variants[0].Weight == nil || variants[1].Weight == nil {
		t.Fatalf("expected variant weight to be preserved: %+v", variants)
	}
	if variants[0].Weight.Value != 120 || variants[1].Weight.Value != 180 {
		t.Fatalf("weights = %+v / %+v", variants[0].Weight, variants[1].Weight)
	}
}
