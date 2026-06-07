package listingkit

import (
	"testing"
	"time"

	"task-processor/internal/catalog/canonical"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestBuildSheinResolutionCacheSummary(t *testing.T) {
	t.Parallel()

	now := time.Now()
	pkg := &SheinPackage{
		CategoryResolution: &SheinCategoryResolution{
			Cache: &sheinpub.ResolutionCacheInfo{
				CacheKey:  "cat-key",
				UpdatedAt: &now,
			},
		},
		AttributeResolution: &SheinAttributeResolution{
			Cache: &sheinpub.ResolutionCacheInfo{
				CacheKey: "attr-key",
			},
		},
		Pricing: &sheinpub.PricingReview{
			Cache: &sheinpub.ResolutionCacheInfo{
				CacheKey: "pricing-key",
			},
		},
	}

	summary := buildSheinResolutionCacheSummary(pkg)
	if summary == nil {
		t.Fatal("expected summary")
	}
	if summary.Category == nil || summary.Category.CacheKey != "cat-key" {
		t.Fatalf("category summary = %+v", summary.Category)
	}
	if summary.Attributes == nil || summary.Attributes.CacheKey != "attr-key" {
		t.Fatalf("attribute summary = %+v", summary.Attributes)
	}
	if summary.SaleAttributes != nil {
		t.Fatalf("sale summary = %+v, want nil", summary.SaleAttributes)
	}
	if summary.Pricing == nil || summary.Pricing.CacheKey != "pricing-key" {
		t.Fatalf("pricing summary = %+v", summary.Pricing)
	}
}

func TestBuildSheinFinalReviewSKU(t *testing.T) {
	t.Parallel()

	sku := SheinSKUDraft{
		SupplierSKU: "SKU-1",
		BasePrice:   "12.50",
		Currency:    "USD",
		StockCount:  8,
		Weight:      0.3,
		SaleAttributes: []SheinResolvedSaleAttribute{
			{Name: "颜色", Value: "Black"},
			{Name: "尺码", Value: "One Size"},
		},
	}

	item := buildSheinFinalReviewSKU("SKC-1", sku)
	if item.SupplierCode != "SKC-1" || item.SupplierSKU != "SKU-1" {
		t.Fatalf("item = %+v", item)
	}
	if item.Color != "Black" || item.Size != "One Size" {
		t.Fatalf("item attrs = %+v", item)
	}
}

func TestResolveSheinFinalReviewImageRole(t *testing.T) {
	t.Parallel()

	role, main := resolveSheinFinalReviewImageRole(
		"https://cdn.example.com/size.jpg",
		"gallery",
		false,
		&sheinpub.FinalDraft{
			ImageRoleOverrides: map[string]string{
				"https://cdn.example.com/skc.jpg": "swatch",
			},
		},
		map[string]struct{}{"https://cdn.example.com/size.jpg": {}},
	)
	if role != "size_map" || main {
		t.Fatalf("role=%q main=%v", role, main)
	}
}

func TestMergeSheinFinalReviewImage(t *testing.T) {
	t.Parallel()

	image := &SheinFinalReviewImage{Role: "gallery"}
	mergeSheinFinalReviewImage(image, "main", true)
	if image.Role != "main" || !image.Main || image.Swatch || image.SizeMap {
		t.Fatalf("image = %+v", image)
	}
}

func TestBuildSheinSourceProductSummary(t *testing.T) {
	t.Parallel()

	product := &canonical.Product{
		Title:        "Bottle",
		CategoryPath: []string{"Home", "Kitchen"},
		Attributes: map[string]canonical.Attribute{
			"sku":   {Value: "SKU-1"},
			"brand": {Value: "Acme"},
		},
	}

	summary := buildSheinSourceProductSummary(product)
	if summary == nil {
		t.Fatal("expected summary")
	}
	if summary.Title != "Bottle" || summary.SKU != "SKU-1" {
		t.Fatalf("summary = %+v", summary)
	}
	if summary.Attributes["brand"] != "Acme" {
		t.Fatalf("attributes = %+v", summary.Attributes)
	}
}
