package listingkit

import (
	"strings"
	"testing"

	"task-processor/internal/asset"
	"task-processor/internal/catalog"
)

func TestGenerateRequestFromProductSourceFactsMapsNeutralFacts(t *testing.T) {
	req := GenerateRequestFromProductSourceFacts(ProductSourceGenerateRequestInput{
		TenantID: " tenant-1 ",
		UserID:   " user-1 ",
		Product: catalog.ProductFacts{
			SourceKey:      "crawler:amazon:B001",
			SourcePlatform: "amazon",
			SourceID:       "B001",
			SourceURL:      " https://www.amazon.com/dp/B001 ",
			Title:          "Test Shirt",
			Brand:          "Test Brand",
			Description:    "Soft shirt",
			Attributes: map[string]string{
				"category": "Clothing>Shirts",
				"asin":     "B001",
			},
			Variants: []catalog.VariantFacts{{SourceID: "B001-BLUE-M"}},
		},
		Assets: asset.Facts{Items: []asset.ItemFacts{
			{URL: " https://img.example/primary.jpg ", Role: "primary"},
			{URL: "https://img.example/primary.jpg", Role: "gallery"},
			{URL: "https://img.example/side.jpg", Role: "gallery"},
		}},
		Platforms:          []string{" SHEIN ", "shein", "amazon"},
		Country:            " US ",
		Language:           " en_US ",
		SheinStoreID:       873,
		TargetCategoryHint: " ",
	})

	if req.TenantID != "tenant-1" || req.UserID != "user-1" {
		t.Fatalf("tenant/user = %q/%q, want trimmed values", req.TenantID, req.UserID)
	}
	if req.ProductURL != "https://www.amazon.com/dp/B001" {
		t.Fatalf("ProductURL = %q, want source URL", req.ProductURL)
	}
	if len(req.ImageURLs) != 2 || req.ImageURLs[0] != "https://img.example/primary.jpg" || req.ImageURLs[1] != "https://img.example/side.jpg" {
		t.Fatalf("ImageURLs = %#v, want deduped source asset URLs", req.ImageURLs)
	}
	if len(req.Platforms) != 2 || req.Platforms[0] != "shein" || req.Platforms[1] != "amazon" {
		t.Fatalf("Platforms = %#v, want normalized deduped order", req.Platforms)
	}
	if req.BrandHint != "Test Brand" {
		t.Fatalf("BrandHint = %q, want product brand", req.BrandHint)
	}
	if req.TargetCategoryHint != "Clothing>Shirts" {
		t.Fatalf("TargetCategoryHint = %q, want category attribute fallback", req.TargetCategoryHint)
	}
	if req.SheinStoreID != 873 {
		t.Fatalf("SheinStoreID = %d, want 873", req.SheinStoreID)
	}
	for _, want := range []string{"Title: Test Shirt", "Brand: Test Brand", "Description: Soft shirt", "Attribute asin: B001", "Attribute category: Clothing>Shirts", "Variant count: 1"} {
		if !strings.Contains(req.Text, want) {
			t.Fatalf("Text = %q, missing %q", req.Text, want)
		}
	}
}

func TestGenerateRequestFromProductSourceFactsKeepsExplicitCategoryHint(t *testing.T) {
	req := GenerateRequestFromProductSourceFacts(ProductSourceGenerateRequestInput{
		Product: catalog.ProductFacts{Attributes: map[string]string{"category": "Source Category"}},
		TargetCategoryHint: " Explicit Category ",
	})

	if req.TargetCategoryHint != "Explicit Category" {
		t.Fatalf("TargetCategoryHint = %q, want explicit category", req.TargetCategoryHint)
	}
}

func TestGenerateRequestFromProductSourceFactsDoesNotRequireAssetsOrPlatforms(t *testing.T) {
	req := GenerateRequestFromProductSourceFacts(ProductSourceGenerateRequestInput{
		Product: catalog.ProductFacts{Title: "Only Title"},
	})

	if req.Text != "Title: Only Title" {
		t.Fatalf("Text = %q, want title-only prompt", req.Text)
	}
	if req.ImageURLs != nil {
		t.Fatalf("ImageURLs = %#v, want nil", req.ImageURLs)
	}
	if req.Platforms != nil {
		t.Fatalf("Platforms = %#v, want nil", req.Platforms)
	}
}
