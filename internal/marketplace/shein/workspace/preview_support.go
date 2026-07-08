package workspace

import (
	"fmt"
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

type ResolutionCacheSummary struct {
	Category       *sheinpub.ResolutionCacheInfo `json:"category,omitempty"`
	Attributes     *sheinpub.ResolutionCacheInfo `json:"attributes,omitempty"`
	SaleAttributes *sheinpub.ResolutionCacheInfo `json:"sale_attributes,omitempty"`
	SizeAttributes *sheinpub.ResolutionCacheInfo `json:"size_attributes,omitempty"`
	Pricing        *sheinpub.ResolutionCacheInfo `json:"pricing,omitempty"`
}

type ImageUploadPreflight struct {
	TotalImageReferences int      `json:"total_image_references"`
	UniqueImageURLs      int      `json:"unique_image_urls"`
	PendingUploadURLs    int      `json:"pending_upload_urls"`
	SheinUploadedURLs    int      `json:"shein_uploaded_urls"`
	SDSMockupURLs        int      `json:"sds_mockup_urls"`
	UsesSDSMockups       bool     `json:"uses_sds_mockups"`
	ReadyForUpload       bool     `json:"ready_for_upload"`
	Summary              []string `json:"summary,omitempty"`
}

type ImageUploadClassifier func(url string) bool
type ImageUploadCacheHit func(pkg *sheinpub.Package, url string) bool

func BuildResolutionCacheSummary(pkg *sheinpub.Package) *ResolutionCacheSummary {
	if pkg == nil {
		return nil
	}
	summary := &ResolutionCacheSummary{}
	if pkg.CategoryResolution != nil {
		summary.Category = sheinpub.CloneResolutionCacheInfo(pkg.CategoryResolution.Cache)
		enrichCategoryResolutionCacheInfo(summary.Category, pkg.CategoryResolution)
	}
	if pkg.AttributeResolution != nil {
		summary.Attributes = sheinpub.CloneResolutionCacheInfo(pkg.AttributeResolution.Cache)
		enrichAttributeResolutionCacheInfo(summary.Attributes, pkg.AttributeResolution)
	}
	if pkg.SaleAttributeResolution != nil {
		summary.SaleAttributes = sheinpub.CloneResolutionCacheInfo(pkg.SaleAttributeResolution.Cache)
		enrichSaleAttributeResolutionCacheInfo(summary.SaleAttributes, pkg.SaleAttributeResolution)
	}
	if pkg.Pricing != nil {
		summary.Pricing = sheinpub.CloneResolutionCacheInfo(pkg.Pricing.Cache)
		enrichPricingResolutionCacheInfo(summary.Pricing, pkg.Pricing)
	}
	if pkg.SizeAttributes != nil {
		summary.SizeAttributes = sheinpub.CloneResolutionCacheInfo(pkg.SizeAttributes.Cache)
		enrichSizeAttributeResolutionCacheInfo(summary.SizeAttributes, pkg.SizeAttributes)
	}
	if summary.Category == nil && summary.Attributes == nil && summary.SaleAttributes == nil && summary.SizeAttributes == nil && summary.Pricing == nil {
		return nil
	}
	return summary
}

func BuildImageUploadPreflight(
	pkg *sheinpub.Package,
	isUploaded ImageUploadClassifier,
	cacheHit ImageUploadCacheHit,
	isSDS ImageUploadClassifier,
) *ImageUploadPreflight {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.PreviewPayload == nil {
		return nil
	}
	urls := collectProductImageURLs(pkg.PreviewPayload)
	if len(urls) == 0 {
		return &ImageUploadPreflight{
			ReadyForUpload: false,
			Summary:        []string{"SHEIN preview payload has no image_info URLs to submit."},
		}
	}

	uniqueURLs := uniqueStrings(urls)
	report := &ImageUploadPreflight{
		TotalImageReferences: len(urls),
		UniqueImageURLs:      len(uniqueURLs),
		ReadyForUpload:       true,
	}
	for _, url := range uniqueURLs {
		switch {
		case isUploaded != nil && isUploaded(url):
			report.SheinUploadedURLs++
		case cacheHit != nil && cacheHit(pkg, url):
			report.SheinUploadedURLs++
		default:
			report.PendingUploadURLs++
		}
		if isSDS != nil && isSDS(url) {
			report.SDSMockupURLs++
		}
	}
	report.UsesSDSMockups = report.SDSMockupURLs > 0
	report.Summary = buildImageUploadPreflightSummary(report)
	return report
}

func collectProductImageURLs(product *sheinproduct.Product) []string {
	if product == nil {
		return nil
	}
	urls := make([]string, 0, 16)
	urls = appendImageInfoURLs(urls, product.ImageInfo)
	for i := range product.SKCList {
		urls = appendImageInfoURLs(urls, &product.SKCList[i].ImageInfo)
		for j := range product.SKCList[i].SKUS {
			urls = appendImageInfoURLs(urls, product.SKCList[i].SKUS[j].ImageInfo)
		}
	}
	return urls
}

func appendImageInfoURLs(urls []string, info *sheinproduct.ImageInfo) []string {
	if info == nil {
		return urls
	}
	for _, image := range info.ImageInfoList {
		if url := strings.TrimSpace(image.ImageURL); url != "" {
			urls = append(urls, url)
		}
	}
	return urls
}

func buildImageUploadPreflightSummary(report *ImageUploadPreflight) []string {
	if report == nil {
		return nil
	}
	summary := []string{
		fmt.Sprintf("%d image references will be submitted across SPU/SKC/SKU.", report.TotalImageReferences),
		fmt.Sprintf("%d unique image URLs need upload de-duplication.", report.UniqueImageURLs),
	}
	if report.PendingUploadURLs > 0 {
		summary = append(summary, fmt.Sprintf("%d unique image URLs will be uploaded to SHEIN before submit.", report.PendingUploadURLs))
	}
	if report.UsesSDSMockups {
		summary = append(summary, fmt.Sprintf("%d unique SDS rendered mockup URLs are present in the SHEIN payload.", report.SDSMockupURLs))
	}
	return summary
}

func enrichCategoryResolutionCacheInfo(info *sheinpub.ResolutionCacheInfo, resolution *sheinpub.CategoryResolution) {
	if info == nil || resolution == nil {
		return
	}
	info.DisplayValue = strings.TrimSpace(strings.Join(resolution.MatchedPath, " > "))
}

func enrichAttributeResolutionCacheInfo(info *sheinpub.ResolutionCacheInfo, resolution *sheinpub.AttributeResolution) {
	if info == nil || resolution == nil {
		return
	}
	parts := make([]string, 0, 4)
	if resolution.ResolvedCount > 0 {
		parts = append(parts, fmt.Sprintf("已解析 %d 个", resolution.ResolvedCount))
	}
	for _, item := range resolution.ResolvedAttributes {
		name := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(item.Value)
		if name == "" || value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", name, value))
		if len(parts) >= 4 {
			break
		}
	}
	if resolution.UnresolvedCount > 0 {
		parts = append(parts, fmt.Sprintf("待补充 %d 个", resolution.UnresolvedCount))
	}
	info.DisplayValue = strings.TrimSpace(strings.Join(parts, "；"))
}

func enrichSaleAttributeResolutionCacheInfo(info *sheinpub.ResolutionCacheInfo, resolution *sheinpub.SaleAttributeResolution) {
	if info == nil || resolution == nil {
		return
	}
	if len(resolution.SelectionSummary) > 0 {
		info.DisplayValue = strings.TrimSpace(strings.Join(resolution.SelectionSummary, "；"))
		return
	}
	parts := make([]string, 0, 4)
	for _, item := range resolution.SKCAttributes {
		name := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(item.Value)
		if name == "" || value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("SKC %s=%s", name, value))
		if len(parts) >= 2 {
			break
		}
	}
	for _, item := range resolution.SKUAttributes {
		name := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(item.Value)
		if name == "" || value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("SKU %s=%s", name, value))
		if len(parts) >= 4 {
			break
		}
	}
	info.DisplayValue = strings.TrimSpace(strings.Join(parts, "；"))
}

func enrichPricingResolutionCacheInfo(info *sheinpub.ResolutionCacheInfo, review *sheinpub.PricingReview) {
	if info == nil || review == nil {
		return
	}
	if info.UpdatedAt == nil && review.UpdatedAt != nil {
		updatedAt := *review.UpdatedAt
		info.UpdatedAt = &updatedAt
	}
	if len(review.SKUPrices) == 0 {
		return
	}
	count := 0
	minPrice := 0.0
	maxPrice := 0.0
	currency := ""
	for _, item := range review.SKUPrices {
		if item.FinalPrice <= 0 {
			continue
		}
		count++
		if currency == "" {
			currency = strings.ToUpper(strings.TrimSpace(item.Currency))
		}
		if minPrice == 0 || item.FinalPrice < minPrice {
			minPrice = item.FinalPrice
		}
		if item.FinalPrice > maxPrice {
			maxPrice = item.FinalPrice
		}
	}
	if count == 0 {
		return
	}
	if currency == "" {
		currency = "PRICE"
	}
	if minPrice == maxPrice {
		info.DisplayValue = fmt.Sprintf("%d SKU；%s %.2f", count, currency, minPrice)
		return
	}
	info.DisplayValue = fmt.Sprintf("%d SKU；%s %.2f - %.2f", count, currency, minPrice, maxPrice)
}

func enrichSizeAttributeResolutionCacheInfo(info *sheinpub.ResolutionCacheInfo, review *sheinpub.SizeAttributeReview) {
	if info == nil || review == nil {
		return
	}
	if info.UpdatedAt == nil && review.UpdatedAt != nil {
		updatedAt := *review.UpdatedAt
		info.UpdatedAt = &updatedAt
	}
	if len(review.Attributes) == 0 {
		return
	}
	info.DisplayValue = fmt.Sprintf("%d 个尺码表值", len(review.Attributes))
}
