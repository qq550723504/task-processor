package sourcing

import (
	"testing"

	"task-processor/internal/model"
)

func TestAmazonSourceEnvelopeMapsProductFacts(t *testing.T) {
	envelope := AmazonSourceEnvelope(AmazonSourceEnvelopeInput{
		Request: SourceRequest{Region: " UK ", ProductID: "fallback", StoreID: 7},
		Product: &model.Product{
			Asin:        " B001 ",
			ParentAsin:  " PARENT ",
			URL:         " https://www.amazon.co.uk/dp/B001 ",
			Title:       " Test Shirt ",
			Brand:       " Test Brand ",
			Description: " Test description ",
			Currency:    "GBP",
			FinalPrice:  12.34,
			SellerID:    " seller-1 ",
			SellerName:  " Seller One ",
			ImageURL:    " https://img.example/primary.jpg ",
			Images:      []string{"https://img.example/primary.jpg", " https://img.example/side.jpg "},
			Features:    []string{" Soft ", " Washable "},
			Categories:  []string{" Clothing ", " Shirts "},
			Variations: []model.Variation{{
				Name:       "Blue / M",
				Asin:       "B001-BLUE-M",
				Attributes: map[string]any{"Color": "Blue", "Size": "M"},
			}},
		},
		RawSnapshot: "raw-1",
		SourceRunID: "run-1",
		RequestID:   "request-1",
	})

	if envelope.Identity.SourceType != SourceTypeCrawler {
		t.Fatalf("SourceType = %q, want crawler", envelope.Identity.SourceType)
	}
	if envelope.Identity.SourcePlatform != AmazonSourcePlatform {
		t.Fatalf("SourcePlatform = %q, want amazon", envelope.Identity.SourcePlatform)
	}
	if envelope.Identity.SourceID != "B001" {
		t.Fatalf("SourceID = %q, want B001", envelope.Identity.SourceID)
	}
	if got := envelope.Identity.Key(); got != "amazon:uk:B001:7" {
		t.Fatalf("Key() = %q, want legacy key with store", got)
	}
	if got := envelope.Identity.SourceKey(); got != "crawler:amazon:B001" {
		t.Fatalf("SourceKey() = %q, want source key", got)
	}
	if envelope.RawReference.ReferenceType != amazonSourceReferenceType || envelope.RawReference.ReferenceID != "B001" {
		t.Fatalf("RawReference = %+v, want Amazon product reference", envelope.RawReference)
	}
	if envelope.ProductCandidate.Title != "Test Shirt" {
		t.Fatalf("Title = %q, want Test Shirt", envelope.ProductCandidate.Title)
	}
	if envelope.ProductCandidate.Attributes["categories"] != "Clothing>Shirts" {
		t.Fatalf("categories = %q, want normalized category path", envelope.ProductCandidate.Attributes["categories"])
	}
	if len(envelope.ProductCandidate.Variants) != 1 {
		t.Fatalf("variants = %d, want 1", len(envelope.ProductCandidate.Variants))
	}
	if envelope.ProductCandidate.Variants[0].Attributes["Color"] != "Blue" {
		t.Fatalf("variant color = %q, want Blue", envelope.ProductCandidate.Variants[0].Attributes["Color"])
	}
	if len(envelope.AssetCandidates) != 2 {
		t.Fatalf("assets = %d, want deduped primary + gallery", len(envelope.AssetCandidates))
	}
	if envelope.AssetCandidates[0].Role != amazonImageRolePrimary {
		t.Fatalf("first asset role = %q, want primary", envelope.AssetCandidates[0].Role)
	}
	if envelope.SupplierOrCostFacts.SupplierID != "seller-1" || envelope.SupplierOrCostFacts.Price != "12.34" {
		t.Fatalf("SupplierOrCostFacts = %+v, want seller and price", envelope.SupplierOrCostFacts)
	}
	if len(envelope.Warnings) != 0 {
		t.Fatalf("Warnings = %+v, want none", envelope.Warnings)
	}
}

func TestAmazonSourceEnvelopeFallsBackToRequestIdentityAndWarnings(t *testing.T) {
	envelope := AmazonSourceEnvelope(AmazonSourceEnvelopeInput{
		Request: SourceRequest{Region: "us", ProductID: "B-FALLBACK"},
		Product: &model.Product{},
	})

	if envelope.Identity.SourceID != "B-FALLBACK" {
		t.Fatalf("SourceID = %q, want request fallback", envelope.Identity.SourceID)
	}
	if len(envelope.Warnings) != 2 {
		t.Fatalf("Warnings = %+v, want missing title and assets", envelope.Warnings)
	}
	codes := map[string]bool{}
	for _, warning := range envelope.Warnings {
		codes[warning.Code] = true
	}
	if !codes["missing_title"] || !codes["missing_assets"] {
		t.Fatalf("warning codes = %+v, want missing_title and missing_assets", codes)
	}
}

func TestAmazonSourceEnvelopeHandlesMissingProduct(t *testing.T) {
	envelope := AmazonSourceEnvelope(AmazonSourceEnvelopeInput{
		Request: SourceRequest{Region: "us", ProductID: "B001"},
	})

	if envelope.Identity.SourceID != "B001" {
		t.Fatalf("SourceID = %q, want request identity", envelope.Identity.SourceID)
	}
	if len(envelope.Warnings) != 1 || envelope.Warnings[0].Code != "missing_product" {
		t.Fatalf("Warnings = %+v, want missing_product", envelope.Warnings)
	}
}
