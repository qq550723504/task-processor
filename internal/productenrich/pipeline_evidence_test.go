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
