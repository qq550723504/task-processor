package enrich

import (
	"testing"

	productenrich "task-processor/internal/productenrich"
)

func TestApplySourceBackedAttributesOverridesCoreFieldsFromScrapedData(t *testing.T) {
	result := &productenrich.ProductJSON{
		Title:       "Portable Mini Washing Machine",
		Description: "Portable Mini Washing Machine",
		Category:    []string{"General", "Product"},
		Attributes: map[string]string{
			"material": "plastic",
		},
		SellingPoints: []string{"bad generated point"},
	}

	analysis := &productenrich.ProductAnalysis{
		ScrapedData: &productenrich.ScrapedData{
			Title:       "Outdoor Waterproof Sunscreen Chair Cushion",
			Category:    "家居饰品 > 户外用品 > 户外坐垫",
			Description: "Outdoor bench cushion for hanging chairs and balcony seating.",
			Images:      []string{"https://example.com/a.jpg"},
			Specs: map[string]string{
				"产品类别": "椅垫",
				"材质":   "涤纶",
			},
			VariantDimensions: []productenrich.ScrapedVariantDimension{{
				Name:   "颜色",
				Values: []string{"深蓝", "黑色"},
			}},
			Variants: []productenrich.ProductVariant{{
				SKU: "SCRAPED-1",
				Attributes: map[string]string{
					"颜色": "深蓝",
				},
			}},
		},
	}

	applySourceBackedAttributes(result, analysis)

	if result.Title != "Outdoor Waterproof Sunscreen Chair Cushion" {
		t.Fatalf("title = %q", result.Title)
	}
	if result.Description != "Outdoor bench cushion for hanging chairs and balcony seating." {
		t.Fatalf("description = %q", result.Description)
	}
	if len(result.Category) != 3 || result.Category[2] != "户外坐垫" {
		t.Fatalf("category = %#v", result.Category)
	}
	if len(result.Images) != 1 || result.Images[0] != "https://example.com/a.jpg" {
		t.Fatalf("images = %#v", result.Images)
	}
	if len(result.VariantDimensions) != 1 || result.VariantDimensions[0].Name != "颜色" {
		t.Fatalf("variant dimensions = %#v", result.VariantDimensions)
	}
	if len(result.Variants) != 1 || result.Variants[0].SKU != "SCRAPED-1" {
		t.Fatalf("variants = %#v", result.Variants)
	}
	if result.Attributes["产品类别"] != "椅垫" || result.Attributes["材质"] != "涤纶" {
		t.Fatalf("attributes = %#v", result.Attributes)
	}
}
