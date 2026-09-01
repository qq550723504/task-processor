package sourcehandoff

import (
	"testing"

	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

func TestCatalogProductFactsFromEnvelopeMapsNeutralFacts(t *testing.T) {
	envelope := sourcing.SourceEnvelope{
		Identity: sourcing.SourceIdentity{
			SourceType:     sourcing.SourceTypeCrawler,
			SourcePlatform: "amazon",
			SourceID:       "B001",
			SourceURL:      "https://www.amazon.com/dp/B001",
		},
		ProductCandidate: sourcing.ProductCandidate{
			Title:       "Test Shirt",
			Description: "Test description",
			Brand:       "Test Brand",
			Attributes:  map[string]string{"asin": "B001", "category": "Shirts"},
			Variants: []sourcing.ProductVariantCandidate{{
				SourceID:   "B001-BLUE-M",
				Title:      "Blue / M",
				SKU:        "SKU-1",
				Attributes: map[string]string{"Color": "Blue", "Size": "M"},
			}},
		},
		Warnings: []sourcing.SourceWarning{{Code: " Missing_Description ", Field: "description", Message: " description is weak "}},
	}

	facts := catalogProductFactsFromEnvelope(envelope)
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

func TestCatalogProductFactsFromSnapshotConsumesCanonicalProductProjection(t *testing.T) {
	snapshot := catalog.ProductSnapshot{
		Title:       "Snapshot title",
		Brand:       "Snapshot brand",
		Description: "Snapshot description",
		Attributes: []catalog.Attribute{
			{Name: "category", Value: "Shirts"},
			{Name: "material", Value: "Cotton"},
		},
		Variants: []catalog.Variant{{
			SourceID: "B001-BLUE-M",
			Title:    "Blue / M",
			SKU:      "SKU-1",
			Attributes: []catalog.Attribute{
				{Name: "Color", Value: "Blue"},
				{Name: "Size", Value: "M"},
			},
		}},
	}
	identity := sourcing.SourceIdentity{
		SourceType:     sourcing.SourceTypeCrawler,
		SourcePlatform: "amazon",
		SourceID:       "B001",
		SourceURL:      "https://www.amazon.com/dp/B001",
	}

	facts := catalogProductFactsFromSnapshot(identity, snapshot, nil)

	if facts.Attributes["category"] != "Shirts" || facts.Attributes["material"] != "Cotton" {
		t.Fatalf("Attributes = %+v, want snapshot attributes", facts.Attributes)
	}
	if len(facts.Variants) != 1 || facts.Variants[0].SourceID != "B001-BLUE-M" || facts.Variants[0].Title != "Blue / M" || facts.Variants[0].SKU != "SKU-1" {
		t.Fatalf("Variants = %+v, want snapshot variant identity", facts.Variants)
	}
	if facts.Variants[0].Attributes["Color"] != "Blue" || facts.Variants[0].Attributes["Size"] != "M" {
		t.Fatalf("Variant attributes = %+v, want snapshot variant attributes", facts.Variants[0].Attributes)
	}
}

func TestAssetFactsFromEnvelopeMapsNeutralAssets(t *testing.T) {
	envelope := sourcing.SourceEnvelope{
		Identity: sourcing.SourceIdentity{
			SourceType:     sourcing.SourceTypeCrawler,
			SourcePlatform: "amazon",
			SourceID:       "B001",
		},
		AssetCandidates: []sourcing.AssetCandidate{{
			SourceID:  "img-1",
			URL:       "https://img.example/1.jpg",
			MediaType: "image",
			Role:      "primary",
			Checksum:  "sha256:1",
		}},
		Warnings: []sourcing.SourceWarning{{Code: " Missing_Alt_Text ", Field: "images", Message: " image alt text missing "}},
	}

	facts := assetFactsFromEnvelope(envelope)
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
