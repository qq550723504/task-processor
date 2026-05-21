package listingkit

import (
	"testing"
	"time"

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
