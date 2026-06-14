package workspace

import (
	"testing"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestBuildResolutionCacheSummary(t *testing.T) {
	t.Parallel()

	now := time.Now()
	pkg := &sheinpub.Package{
		CategoryResolution: &sheinpub.CategoryResolution{
			MatchedPath: []string{"Home", "Decor", "Wall Art"},
			Cache: &sheinpub.ResolutionCacheInfo{
				CacheKey:  "cat-key",
				UpdatedAt: &now,
			},
		},
		AttributeResolution: &sheinpub.AttributeResolution{
			ResolvedCount:   2,
			UnresolvedCount: 1,
			ResolvedAttributes: []sheinpub.ResolvedAttribute{
				{Name: "Material", Value: "Metal"},
				{Name: "Style", Value: "Vintage"},
			},
			Cache: &sheinpub.ResolutionCacheInfo{CacheKey: "attr-key"},
		},
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			SelectionSummary: []string{"主属性：尺寸", "未选择第二属性"},
			Cache:            &sheinpub.ResolutionCacheInfo{CacheKey: "sale-key"},
		},
		Pricing: &sheinpub.PricingReview{
			UpdatedAt: &now,
			Cache:     &sheinpub.ResolutionCacheInfo{CacheKey: "pricing-key"},
			SKUPrices: []sheinpub.SKUPriceReview{
				{FinalPrice: 19.99, Currency: "USD"},
				{FinalPrice: 27.99, Currency: "USD"},
			},
		},
	}

	summary := BuildResolutionCacheSummary(pkg)
	if summary == nil {
		t.Fatal("expected summary")
	}
	if summary.Category == nil || summary.Category.DisplayValue != "Home > Decor > Wall Art" {
		t.Fatalf("category summary = %+v", summary.Category)
	}
	if summary.Attributes == nil || summary.Attributes.DisplayValue == "" {
		t.Fatalf("attribute summary = %+v", summary.Attributes)
	}
	if summary.SaleAttributes == nil || summary.SaleAttributes.DisplayValue != "主属性：尺寸；未选择第二属性" {
		t.Fatalf("sale summary = %+v", summary.SaleAttributes)
	}
	if summary.Pricing == nil || summary.Pricing.DisplayValue != "2 SKU；USD 19.99 - 27.99" {
		t.Fatalf("pricing summary = %+v", summary.Pricing)
	}
}

func TestBuildImageUploadPreflight(t *testing.T) {
	t.Parallel()

	rendered := []string{
		"https://cdn.sdspod.com/out/0/202604/rendered-main.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-1.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-2.jpg",
	}
	pkg := &sheinpub.Package{
		PreviewPayload: &sheinproduct.Product{
			ImageInfo: imageInfo(rendered),
			SKCList: []sheinproduct.SKC{
				{
					ImageInfo: *imageInfo(rendered),
					SKUS: []sheinproduct.SKU{
						{ImageInfo: imageInfo(rendered[:1])},
					},
				},
			},
		},
	}

	report := BuildImageUploadPreflight(
		pkg,
		func(url string) bool { return false },
		func(pkg *sheinpub.Package, url string) bool { return false },
		func(url string) bool { return true },
	)
	if report == nil {
		t.Fatal("expected report")
	}
	if report.TotalImageReferences != 7 {
		t.Fatalf("total references = %d, want 7", report.TotalImageReferences)
	}
	if report.UniqueImageURLs != len(rendered) {
		t.Fatalf("unique urls = %d, want %d", report.UniqueImageURLs, len(rendered))
	}
	if report.PendingUploadURLs != len(rendered) {
		t.Fatalf("pending upload urls = %d, want %d", report.PendingUploadURLs, len(rendered))
	}
	if !report.UsesSDSMockups || report.SDSMockupURLs != len(rendered) {
		t.Fatalf("report = %+v, want SDS mockup counts", report)
	}
}

func imageInfo(urls []string) *sheinproduct.ImageInfo {
	info := &sheinproduct.ImageInfo{
		ImageInfoList: make([]sheinproduct.ImageDetail, 0, len(urls)),
	}
	for index, url := range urls {
		imageType := 2
		if index == 0 {
			imageType = 1
		}
		info.ImageInfoList = append(info.ImageInfoList, sheinproduct.ImageDetail{
			ImageType:          imageType,
			ImageSort:          index + 1,
			ImageURL:           url,
			MarketingMainImage: index == 0,
		})
	}
	return info
}
