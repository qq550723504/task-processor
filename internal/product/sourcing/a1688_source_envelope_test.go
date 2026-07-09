package sourcing

import (
	"errors"
	"testing"

	alibaba1688model "task-processor/internal/crawler/alibaba1688/model"
)

func TestAlibaba1688SourceEnvelopeMapsProductFacts(t *testing.T) {
	envelope := Alibaba1688SourceEnvelope(Alibaba1688SourceEnvelopeInput{
		Request: Alibaba1688CrawlRequestInput{URL: "https://detail.1688.com/offer/123.html?spm=test", StoreID: 9},
		Product: &alibaba1688model.Product1688{
			ID:               "123",
			Title:            "Canvas Tote Bag",
			URL:              "https://detail.1688.com/offer/123.html?foo=bar",
			MainImage:        " https://img.example/main.jpg ",
			Images:           []string{"https://img.example/main.jpg", "https://img.example/gallery.jpg"},
			MinPrice:         8.5,
			MaxPrice:         12.0,
			Currency:         "CNY",
			MinOrderQuantity: 2,
			Unit:             "件",
			Category:         "Bags",
			Brand:            "Factory Brand",
			Keywords:         []string{" tote ", "canvas"},
			IsCustomized:     true,
			SalesVolume:      100,
			ReviewCount:      8,
			Rating:           4.8,
			Supplier: alibaba1688model.SupplierInfo{
				ID:              "supplier-1",
				Name:            "Supplier One",
				CompanyName:     "Supplier Co",
				Location:        "Guangdong",
				ShopURL:         "https://shop.1688.com/supplier-1",
				CardType:        "factory",
				YearsInBusiness: 5,
				Rating:          4.7,
				ResponseRate:    98.5,
				IsGoldSupplier:  true,
				IsVerified:      true,
			},
			Specifications: []alibaba1688model.Specification{{Name: "Material", Value: "Canvas"}},
			ProductDetails: []alibaba1688model.ProductDetail{{Content: "Durable bag", Images: []string{"https://img.example/detail.jpg"}}},
			PackInfo: &alibaba1688model.PackInfo{
				PackageType:     "box",
				Weight:          500,
				PackageImages:   []string{"https://img.example/pack.jpg"},
				PackageContents: []string{"bag"},
			},
			Variants: []alibaba1688model.Variant{{
				Name:       "Blue / M",
				Image:      "https://img.example/variant.jpg",
				Stock:      20,
				Price:      9.9,
				Attributes: map[string]any{"Color": "Blue", "Size": "M"},
			}},
			Videos: []alibaba1688model.Video{{VideoURL: "https://video.example/1.mp4", CoverURL: "https://img.example/video-cover.jpg"}},
			ShippingInfo: alibaba1688model.ShippingInfo{ShippingFrom: "Guangdong", ProcessingTime: "3 days"},
		},
		RawSnapshot: "raw-1688-1",
		SourceRunID: "run-1",
		RequestID:   "request-1",
	})

	if envelope.Identity.SourceType != SourceTypeCrawler {
		t.Fatalf("SourceType = %q, want crawler", envelope.Identity.SourceType)
	}
	if envelope.Identity.SourcePlatform != Alibaba1688SourcePlatform {
		t.Fatalf("SourcePlatform = %q, want 1688", envelope.Identity.SourcePlatform)
	}
	if envelope.Identity.SourceID != "123" {
		t.Fatalf("SourceID = %q, want 123", envelope.Identity.SourceID)
	}
	if got := envelope.Identity.Key(); got != "1688:cn:123:9" {
		t.Fatalf("Key() = %q, want legacy key with store", got)
	}
	if got := envelope.Identity.SourceKey(); got != "crawler:1688:123" {
		t.Fatalf("SourceKey() = %q, want source key", got)
	}
	if envelope.RawReference.ReferenceType != alibaba1688SourceReferenceType || envelope.RawReference.ReferenceID != "123" {
		t.Fatalf("RawReference = %+v, want 1688 product reference", envelope.RawReference)
	}
	if envelope.RawReference.URL != "https://detail.1688.com/offer/123.html" {
		t.Fatalf("RawReference.URL = %q, want normalized URL without query", envelope.RawReference.URL)
	}
	if envelope.ProductCandidate.Title != "Canvas Tote Bag" || envelope.ProductCandidate.Brand != "Factory Brand" {
		t.Fatalf("ProductCandidate = %+v, want title and brand", envelope.ProductCandidate)
	}
	if envelope.ProductCandidate.Description != "Durable bag" {
		t.Fatalf("Description = %q, want product detail content", envelope.ProductCandidate.Description)
	}
	if envelope.ProductCandidate.Attributes["spec:Material"] != "Canvas" {
		t.Fatalf("Material spec = %q, want Canvas", envelope.ProductCandidate.Attributes["spec:Material"])
	}
	if envelope.ProductCandidate.Attributes["keywords"] != "tote,canvas" {
		t.Fatalf("keywords = %q, want normalized keywords", envelope.ProductCandidate.Attributes["keywords"])
	}
	if len(envelope.ProductCandidate.Variants) != 1 {
		t.Fatalf("variants = %d, want 1", len(envelope.ProductCandidate.Variants))
	}
	variant := envelope.ProductCandidate.Variants[0]
	if variant.Attributes["Color"] != "Blue" || variant.Attributes["price"] != "9.9" || variant.Attributes["stock"] != "20" {
		t.Fatalf("variant attributes = %+v, want color price and stock", variant.Attributes)
	}
	if len(envelope.AssetCandidates) != 7 {
		t.Fatalf("assets = %d, want main/gallery/detail/variant/package/video cover/video", len(envelope.AssetCandidates))
	}
	if envelope.AssetCandidates[0].Role != alibaba1688ImageRolePrimary {
		t.Fatalf("first asset role = %q, want primary", envelope.AssetCandidates[0].Role)
	}
	if envelope.SupplierOrCostFacts.SupplierID != "supplier-1" || envelope.SupplierOrCostFacts.Price != "8.5" {
		t.Fatalf("SupplierOrCostFacts = %+v, want supplier and min price", envelope.SupplierOrCostFacts)
	}
	if envelope.SupplierOrCostFacts.Facts["is_gold_supplier"] != "true" {
		t.Fatalf("is_gold_supplier = %q, want true", envelope.SupplierOrCostFacts.Facts["is_gold_supplier"])
	}
	if len(envelope.Warnings) != 0 {
		t.Fatalf("Warnings = %+v, want none", envelope.Warnings)
	}
}

func TestAlibaba1688SourceEnvelopeFallsBackToRequestIdentityAndWarnings(t *testing.T) {
	envelope := Alibaba1688SourceEnvelope(Alibaba1688SourceEnvelopeInput{
		Request: Alibaba1688CrawlRequestInput{URL: "detail.1688.com/offer/456.html"},
		Product: &alibaba1688model.Product1688{},
	})

	if envelope.Identity.SourceID != "456" {
		t.Fatalf("SourceID = %q, want request offer id", envelope.Identity.SourceID)
	}
	codes := map[string]bool{}
	for _, warning := range envelope.Warnings {
		codes[warning.Code] = true
	}
	for _, want := range []string{"missing_title", "missing_assets", "missing_cost"} {
		if !codes[want] {
			t.Fatalf("warning codes = %+v, missing %s", codes, want)
		}
	}
}

func TestAlibaba1688SourceEnvelopeHandlesMissingProductAndError(t *testing.T) {
	envelope := Alibaba1688SourceEnvelope(Alibaba1688SourceEnvelopeInput{
		Request: Alibaba1688CrawlRequestInput{URL: "https://detail.1688.com/offer/789.html"},
		Error:   errors.New("crawler failed"),
	})

	if envelope.Identity.SourceID != "789" {
		t.Fatalf("SourceID = %q, want request identity", envelope.Identity.SourceID)
	}
	codes := map[string]bool{}
	for _, warning := range envelope.Warnings {
		codes[warning.Code] = true
	}
	if !codes["missing_product"] || !codes["source_error"] {
		t.Fatalf("warning codes = %+v, want missing_product and source_error", codes)
	}
}
