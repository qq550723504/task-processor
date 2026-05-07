package productenrich

import "testing"

func TestAttachProductEvidenceFromScrapedData(t *testing.T) {
	product := &ProductJSON{
		Attributes: map[string]string{
			"material": "ABS",
		},
	}
	input := &ParsedInput{
		ScrapedData: &ScrapedData{
			Title:       "Portable Blender Bottle",
			Description: "USB rechargeable portable blender for smoothies.",
			Specs: map[string]string{
				"material": "ABS Plastic",
				"power":    "50W",
			},
		},
	}

	attachProductEvidence(product, input)

	if len(product.Evidence["title"]) != 1 || product.Evidence["title"][0].Detail != `scraped title: "Portable Blender Bottle"` {
		t.Fatalf("unexpected title evidence: %+v", product.Evidence["title"])
	}
	if len(product.Evidence["description"]) != 1 || product.Evidence["description"][0].Detail != `scraped description: "USB rechargeable portable blender for smoothies."` {
		t.Fatalf("unexpected description evidence: %+v", product.Evidence["description"])
	}
	if len(product.Evidence["attributes.material"]) != 1 || product.Evidence["attributes.material"][0].Detail != "scraped spec material: ABS Plastic" {
		t.Fatalf("unexpected material evidence: %+v", product.Evidence["attributes.material"])
	}
	if len(product.Evidence["specifications.technical.power"]) != 1 || product.Evidence["specifications.technical.power"][0].Detail != "scraped spec power: 50W" {
		t.Fatalf("unexpected power spec evidence: %+v", product.Evidence["specifications.technical.power"])
	}
}

func TestAttachProductEvidenceIncludesSourceBackedStructuralFields(t *testing.T) {
	product := &ProductJSON{}
	input := &ParsedInput{
		ScrapedData: &ScrapedData{
			Category: "家居饰品 > 户外用品",
			Images:   []string{"https://example.com/source.jpg"},
			VariantDimensions: []ScrapedVariantDimension{{
				Name:   "颜色",
				Values: []string{"黑色"},
			}},
			Variants: []ProductVariant{{
				SKU:        "SRC-BLACK",
				Attributes: map[string]string{"颜色": "黑色"},
			}},
		},
	}

	attachProductEvidence(product, input)

	if len(product.Evidence["category_path"]) == 0 {
		t.Fatal("expected category_path evidence from scraped data")
	}
	if len(product.Evidence["images"]) == 0 {
		t.Fatal("expected images evidence from scraped data")
	}
	if len(product.Evidence["variant_dimensions"]) == 0 {
		t.Fatal("expected variant_dimensions evidence from scraped data")
	}
	if len(product.Evidence["variants"]) == 0 {
		t.Fatal("expected variants evidence from scraped data")
	}
}
