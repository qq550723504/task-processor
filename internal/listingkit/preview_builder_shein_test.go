package listingkit

import (
	"slices"
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
			MatchedPath: []string{"Home", "Decor", "Wall Art"},
			Cache: &sheinpub.ResolutionCacheInfo{
				CacheKey:  "cat-key",
				UpdatedAt: &now,
			},
		},
		AttributeResolution: &SheinAttributeResolution{
			ResolvedCount:   2,
			UnresolvedCount: 1,
			ResolvedAttributes: []sheinpub.ResolvedAttribute{
				{Name: "Material", Value: "Metal"},
				{Name: "Style", Value: "Vintage"},
			},
			Cache: &sheinpub.ResolutionCacheInfo{
				CacheKey: "attr-key",
			},
		},
		SaleAttributeResolution: &SheinSaleAttributeResolution{
			SelectionSummary: []string{"主属性：尺寸", "未选择第二属性"},
			Cache: &sheinpub.ResolutionCacheInfo{
				CacheKey: "sale-key",
			},
		},
		Pricing: &sheinpub.PricingReview{
			UpdatedAt: &now,
			Cache: &sheinpub.ResolutionCacheInfo{
				CacheKey: "pricing-key",
			},
			SKUPrices: []sheinpub.SKUPriceReview{
				{FinalPrice: 19.99, Currency: "USD"},
				{FinalPrice: 27.99, Currency: "USD"},
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
	if summary.Category.DisplayValue != "Home > Decor > Wall Art" {
		t.Fatalf("category display value = %q", summary.Category.DisplayValue)
	}
	if summary.Attributes == nil || summary.Attributes.CacheKey != "attr-key" {
		t.Fatalf("attribute summary = %+v", summary.Attributes)
	}
	if summary.Attributes.DisplayValue == "" {
		t.Fatalf("attribute display value = empty, want summary")
	}
	if summary.SaleAttributes == nil || summary.SaleAttributes.CacheKey != "sale-key" {
		t.Fatalf("sale summary = %+v", summary.SaleAttributes)
	}
	if summary.SaleAttributes.DisplayValue != "主属性：尺寸；未选择第二属性" {
		t.Fatalf("sale display value = %q", summary.SaleAttributes.DisplayValue)
	}
	if summary.Pricing == nil || summary.Pricing.CacheKey != "pricing-key" {
		t.Fatalf("pricing summary = %+v", summary.Pricing)
	}
	if summary.Pricing.UpdatedAt == nil || !summary.Pricing.UpdatedAt.Equal(now) {
		t.Fatalf("pricing updated_at = %+v, want %v", summary.Pricing.UpdatedAt, now)
	}
	if summary.Pricing.DisplayValue != "2 SKU；USD 19.99 - 27.99" {
		t.Fatalf("pricing display value = %q", summary.Pricing.DisplayValue)
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

func TestBuildSheinPreviewReviewSummary(t *testing.T) {
	t.Parallel()

	needsReview, summary := buildSheinPreviewReviewSummary(&SheinPackage{
		ReviewNotes: []string{"缺少类目", "缺少类目"},
		Inspection: &sheinpub.Inspection{
			NeedsReview: true,
			Summary:     []string{"图片待确认", "缺少类目"},
		},
	})
	if !needsReview {
		t.Fatal("needsReview = false, want true")
	}
	want := []string{"缺少类目", "图片待确认"}
	if !slices.Equal(summary, want) {
		t.Fatalf("summary = %#v, want %#v", summary, want)
	}
}
