package tests

import (
	"strings"
	"testing"

	alibaba1688model "task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/listingkit"
	"task-processor/internal/product/sourcing"
)

func TestAlibaba1688SourceFactsFlowProducesListingKitRequest(t *testing.T) {
	envelope := sourcing.Alibaba1688SourceEnvelope(sourcing.Alibaba1688SourceEnvelopeInput{
		Request: sourcing.Alibaba1688CrawlRequestInput{URL: "https://detail.1688.com/offer/321.html?spm=flow", StoreID: 11},
		Product: &alibaba1688model.Product1688{
			ID:               "321",
			Title:            "Insulated Lunch Bag",
			URL:              "https://detail.1688.com/offer/321.html?foo=bar",
			MainImage:        "https://img.example/321-main.jpg",
			Images:           []string{"https://img.example/321-main.jpg", "https://img.example/321-side.jpg"},
			MinPrice:         18.8,
			Currency:         "CNY",
			MinOrderQuantity: 3,
			Unit:             "个",
			Category:         "Bags>Lunch Bags",
			Brand:            "Factory Lunch",
			Supplier: alibaba1688model.SupplierInfo{
				ID:   "supplier-321",
				Name: "Lunch Factory",
			},
			Specifications: []alibaba1688model.Specification{{Name: "Material", Value: "Oxford cloth"}},
			ProductDetails: []alibaba1688model.ProductDetail{{
				Content: "Thermal lunch bag with zipper.",
				Images:  []string{"https://img.example/321-detail.jpg"},
			}},
			Variants: []alibaba1688model.Variant{{
				Name:       "Black",
				Image:      "https://img.example/321-black.jpg",
				Stock:      50,
				Price:      19.9,
				Attributes: map[string]any{"Color": "Black"},
			}},
		},
		RawSnapshot: "flow-snapshot-321",
		SourceRunID: "flow-run-1",
		RequestID:   "flow-request-1",
	})

	if got := envelope.Identity.SourceKey(); got != "crawler:1688:321" {
		t.Fatalf("SourceKey() = %q, want crawler:1688:321", got)
	}
	if len(envelope.Warnings) != 0 {
		t.Fatalf("SourceEnvelope warnings = %+v, want none", envelope.Warnings)
	}

	productFacts := sourcing.CatalogProductFactsFromEnvelope(envelope)
	assetFacts := sourcing.AssetFactsFromEnvelope(envelope)

	if !productFacts.HasIdentity() {
		t.Fatal("catalog product facts should preserve source identity")
	}
	if productFacts.SourcePlatform != sourcing.Alibaba1688SourcePlatform || productFacts.SourceID != "321" {
		t.Fatalf("catalog source = %q/%q, want 1688/321", productFacts.SourcePlatform, productFacts.SourceID)
	}
	if productFacts.Title != "Insulated Lunch Bag" {
		t.Fatalf("catalog title = %q, want source title", productFacts.Title)
	}
	if productFacts.Attributes["category"] != "Bags>Lunch Bags" {
		t.Fatalf("catalog category = %q, want source category", productFacts.Attributes["category"])
	}
	if len(productFacts.Variants) != 1 || productFacts.Variants[0].Attributes["Color"] != "Black" {
		t.Fatalf("catalog variants = %+v, want one black variant", productFacts.Variants)
	}
	if !assetFacts.HasAssets() {
		t.Fatal("asset facts should preserve 1688 source images")
	}

	req := listingkit.GenerateRequestFromSourceFacts(listingkit.SourceFactsGenerateRequestInput{
		TenantID:     " tenant-1688 ",
		UserID:       " user-1688 ",
		Product:      productFacts,
		Assets:       assetFacts,
		Platforms:    []string{" SHEIN ", "shein"},
		Country:      " US ",
		Language:     " en_US ",
		SheinStoreID: 168811,
	})

	if req.TenantID != "tenant-1688" || req.UserID != "user-1688" {
		t.Fatalf("request tenant/user = %q/%q, want trimmed values", req.TenantID, req.UserID)
	}
	if req.ProductURL != "https://detail.1688.com/offer/321.html" {
		t.Fatalf("ProductURL = %q, want normalized 1688 source URL", req.ProductURL)
	}
	if req.BrandHint != "Factory Lunch" {
		t.Fatalf("BrandHint = %q, want source brand", req.BrandHint)
	}
	if req.TargetCategoryHint != "Bags>Lunch Bags" {
		t.Fatalf("TargetCategoryHint = %q, want source category", req.TargetCategoryHint)
	}
	if len(req.Platforms) != 1 || req.Platforms[0] != "shein" {
		t.Fatalf("Platforms = %#v, want normalized deduped shein platform", req.Platforms)
	}
	if len(req.ImageURLs) != 4 {
		t.Fatalf("ImageURLs = %#v, want main/side/detail/variant images", req.ImageURLs)
	}
	for _, want := range []string{
		"Title: Insulated Lunch Bag",
		"Brand: Factory Lunch",
		"Description: Thermal lunch bag with zipper.",
		"Attribute category: Bags>Lunch Bags",
		"Attribute min_price: 18.8",
		"Variant count: 1",
	} {
		if !strings.Contains(req.Text, want) {
			t.Fatalf("request text = %q, missing %q", req.Text, want)
		}
	}
}
