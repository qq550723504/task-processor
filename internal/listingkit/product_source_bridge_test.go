package listingkit

import (
	"strings"
	"testing"

	"task-processor/internal/product/catalog"
)

func TestGenerateRequestFromSourceFactsMapsProductIdentityWithoutAssets(t *testing.T) {
	req := GenerateRequestFromSourceFacts(SourceFactsGenerateRequestInput{
		TenantID: " tenant-1 ",
		UserID:   " user-1 ",
		Product: catalog.ProductFacts{
			SourceKey:      "crawler:amazon:B001",
			SourceType:     "crawler",
			SourcePlatform: "amazon",
			SourceID:       "B001",
			SourceURL:      " https://www.amazon.com/dp/B001 ",
			Title:          "Test Shirt",
			Brand:          "Test Brand",
			Description:    "Soft shirt",
			Attributes:     map[string]string{"asin": "B001"},
			Variants:       []catalog.VariantFacts{{SourceID: "B001-BLUE-M"}},
		},
		Platforms:          []string{" SHEIN ", "shein", "amazon"},
		TargetCategoryHint: " Clothing > Shirts ",
	})

	if req.ProductKey != "crawler:amazon:B001" {
		t.Fatalf("ProductKey = %q", req.ProductKey)
	}
	if req.TenantID != "tenant-1" || req.UserID != "user-1" {
		t.Fatalf("tenant/user = %q/%q", req.TenantID, req.UserID)
	}
	if req.Source == nil || req.Source.URL != "https://www.amazon.com/dp/B001" {
		t.Fatalf("Source = %+v", req.Source)
	}
	if req.TargetCategoryHint != "Clothing > Shirts" {
		t.Fatalf("TargetCategoryHint = %q", req.TargetCategoryHint)
	}
	if len(req.Platforms) != 2 || req.Platforms[0] != "shein" || req.Platforms[1] != "amazon" {
		t.Fatalf("Platforms = %#v", req.Platforms)
	}
	for _, want := range []string{"Title: Test Shirt", "Brand: Test Brand", "Attribute asin: B001", "Variant count: 1"} {
		if !strings.Contains(req.Text, want) {
			t.Fatalf("Text = %q, missing %q", req.Text, want)
		}
	}
}

func TestGenerateRequestFromSourceFactsDoesNotGuessCategoryFromAttributes(t *testing.T) {
	req := GenerateRequestFromSourceFacts(SourceFactsGenerateRequestInput{
		Product: catalog.ProductFacts{SourceKey: "product-1", Attributes: map[string]string{"category": "legacy-category"}},
	})
	if req.TargetCategoryHint != "" {
		t.Fatalf("TargetCategoryHint = %q, want explicit input only", req.TargetCategoryHint)
	}
}

func TestGenerateRequestFromSourceFactsAllowsProductWithoutSourceReference(t *testing.T) {
	req := GenerateRequestFromSourceFacts(SourceFactsGenerateRequestInput{Product: catalog.ProductFacts{Title: "Only Title"}})
	if req.Source != nil || req.Text != "Title: Only Title" {
		t.Fatalf("request = %+v", req)
	}
}
