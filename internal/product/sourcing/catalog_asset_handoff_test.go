package sourcing

import "testing"

func TestCatalogProductFactsFromEnvelopeMapsNeutralFacts(t *testing.T) {
	envelope := SourceEnvelope{
		Identity: SourceIdentity{
			SourceType:     SourceTypeCrawler,
			SourcePlatform: AmazonSourcePlatform,
			SourceID:       "B001",
			SourceURL:      "https://www.amazon.com/dp/B001",
		},
		ProductCandidate: ProductCandidate{
			Title:       "Test Shirt",
			Description: "Test description",
			Brand:       "Test Brand",
			Attributes:  map[string]string{"asin": "B001", "category": "Shirts"},
			Variants: []ProductVariantCandidate{{
				SourceID:   "B001-BLUE-M",
				Title:      "Blue / M",
				SKU:        "SKU-1",
				Attributes: map[string]string{"Color": "Blue", "Size": "M"},
			}},
		},
		Warnings: []SourceWarning{{Code: " Missing_Description ", Field: "description", Message: " description is weak "}},
	}

	facts := CatalogProductFactsFromEnvelope(envelope)
	if !facts.HasIdentity() {
		t.Fatal("HasIdentity() = false, want true")
	}
	if facts.SourceKey != "crawler:amazon:B001" {
		t.Fatalf("SourceKey = %q, want crawler:amazon:B001", facts.SourceKey)
	}
	if facts.Title != "Test Shirt" || facts.Brand != "Test Brand" {
		t.Fatalf("facts = %+v, want title and brand", facts)
	}
	if facts.Attributes["asin"] != "B001" {
		t.Fatalf("asin attribute = %q, want B001", facts.Attributes["asin"])
	}
	if len(facts.Variants) != 1 || facts.Variants[0].Attributes["Color"] != "Blue" {
		t.Fatalf("variants = %+v, want mapped variant facts", facts.Variants)
	}
	if len(facts.Warnings) != 1 || facts.Warnings[0].Code != "missing_description" {
		t.Fatalf("warnings = %+v, want normalized warning", facts.Warnings)
	}

	envelope.ProductCandidate.Attributes["asin"] = "mutated"
	envelope.ProductCandidate.Variants[0].Attributes["Color"] = "Red"
	if facts.Attributes["asin"] != "B001" {
		t.Fatalf("facts attributes mutated through source map, got %q", facts.Attributes["asin"])
	}
	if facts.Variants[0].Attributes["Color"] != "Blue" {
		t.Fatalf("variant attributes mutated through source map, got %q", facts.Variants[0].Attributes["Color"])
	}
}

func TestAssetFactsFromEnvelopeMapsNeutralAssets(t *testing.T) {
	envelope := SourceEnvelope{
		Identity: SourceIdentity{
			SourceType:     SourceTypeCrawler,
			SourcePlatform: AmazonSourcePlatform,
			SourceID:       "B001",
		},
		AssetCandidates: []AssetCandidate{{
			SourceID:  "img-1",
			URL:       "https://img.example/1.jpg",
			MediaType: "image",
			Role:      "primary",
			Checksum:  "sha256:1",
		}},
		Warnings: []SourceWarning{{Code: " Missing_Alt_Text ", Field: "images", Message: " image alt text missing "}},
	}

	facts := AssetFactsFromEnvelope(envelope)
	if !facts.HasAssets() {
		t.Fatal("HasAssets() = false, want true")
	}
	if facts.SourceKey != "crawler:amazon:B001" {
		t.Fatalf("SourceKey = %q, want crawler:amazon:B001", facts.SourceKey)
	}
	if len(facts.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(facts.Items))
	}
	if facts.Items[0].URL != "https://img.example/1.jpg" || facts.Items[0].Role != "primary" {
		t.Fatalf("asset item = %+v, want mapped asset facts", facts.Items[0])
	}
	if len(facts.Warnings) != 1 || facts.Warnings[0].Code != "missing_alt_text" {
		t.Fatalf("warnings = %+v, want normalized warning", facts.Warnings)
	}
}
