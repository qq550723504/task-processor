package tests

import (
	"strings"
	"testing"

	"task-processor/internal/compatibility/listingkit/sourcehandoff"
	sourcea1688 "task-processor/internal/integration/crawler/a1688"
	"task-processor/internal/product/sourcing"
)

func TestAlibaba1688SourceFactsFlowProducesListingKitRequest(t *testing.T) {
	envelope := sourcea1688.Alibaba1688SourceEnvelope(sourcea1688.Alibaba1688SourceEnvelopeInput{
		Request: sourcea1688.Alibaba1688CrawlRequestInput{URL: "https://detail.1688.com/offer/321.html?spm=flow", StoreID: 11},
		Product: &sourcea1688.Alibaba1688ProductSnapshot{
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
			Supplier: sourcea1688.Alibaba1688SupplierSnapshot{
				ID:   "supplier-321",
				Name: "Lunch Factory",
			},
			Specifications: []sourcea1688.Alibaba1688SpecificationSnapshot{{Name: "Material", Value: "Oxford cloth"}},
			ProductDetails: []sourcea1688.Alibaba1688ProductDetailSnapshot{{
				Content: "Thermal lunch bag with zipper.",
				Images:  []string{"https://img.example/321-detail.jpg"},
			}},
			Variants: []sourcea1688.Alibaba1688VariantSnapshot{{
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

	snapshot, err := sourcing.ToSnapshot(envelope)
	if err != nil {
		t.Fatalf("ToSnapshot() error = %v", err)
	}
	if snapshot.Title != "Insulated Lunch Bag" || snapshot.Brand != "Factory Lunch" {
		t.Fatalf("snapshot facts = %+v, want source title and brand", snapshot)
	}
	if len(snapshot.Sources) != 1 || snapshot.Sources[0].Type != sourcing.SourceTypeCrawler || snapshot.Sources[0].Detail != "321" {
		t.Fatalf("snapshot sources = %+v, want crawler/raw product lineage", snapshot.Sources)
	}

	req := sourcehandoff.GenerateRequestFromEnvelope(sourcehandoff.ListingKitRequestInput{
		Envelope:           envelope,
		TenantID:           " tenant-1688 ",
		UserID:             " user-1688 ",
		Platforms:          []string{" SHEIN ", "shein"},
		Country:            " US ",
		Language:           " en_US ",
		SheinStoreID:       168811,
		TargetCategoryHint: " Bags>Lunch Bags ",
	})

	if req.TenantID != "tenant-1688" || req.UserID != "user-1688" {
		t.Fatalf("request tenant/user = %q/%q, want trimmed values", req.TenantID, req.UserID)
	}
	if req.ProductKey != "crawler:1688:321" {
		t.Fatalf("ProductKey = %q, want normalized source identity", req.ProductKey)
	}
	if req.BrandHint != "Factory Lunch" {
		t.Fatalf("BrandHint = %q, want source brand", req.BrandHint)
	}
	if req.TargetCategoryHint != "Bags>Lunch Bags" {
		t.Fatalf("TargetCategoryHint = %q, want explicit category hint", req.TargetCategoryHint)
	}
	if len(req.Platforms) != 1 || req.Platforms[0] != "shein" {
		t.Fatalf("Platforms = %#v, want normalized deduped shein platform", req.Platforms)
	}
	if req.Source == nil || req.Source.URL != "https://detail.1688.com/offer/321.html" {
		t.Fatalf("Source = %#v, want normalized source reference", req.Source)
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
