package listingkit

import (
	"fmt"
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func buildSheinImageUploadPreflight(pkg *SheinPackage) *SheinImageUploadPreflight {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.PreviewPayload == nil {
		return nil
	}
	urls := collectSheinProductImageURLs(pkg.PreviewPayload)
	if len(urls) == 0 {
		return &SheinImageUploadPreflight{
			ReadyForUpload: false,
			Summary:        []string{"SHEIN preview payload has no image_info URLs to submit."},
		}
	}

	uniqueURLs := uniqueNonEmptyStrings(urls)
	report := &SheinImageUploadPreflight{
		TotalImageReferences: len(urls),
		UniqueImageURLs:      len(uniqueURLs),
		ReadyForUpload:       true,
	}
	for _, url := range uniqueURLs {
		switch {
		case isSheinUploadedImageURL(url):
			report.SheinUploadedURLs++
		case sheinImageUploadCacheHit(pkg, url):
			report.SheinUploadedURLs++
		default:
			report.PendingUploadURLs++
		}
		if isSDSImageURL(url) {
			report.SDSMockupURLs++
		}
	}
	report.UsesSDSMockups = report.SDSMockupURLs > 0
	report.Summary = buildSheinImageUploadPreflightSummary(report)
	return report
}

func collectSheinProductImageURLs(product *sheinproduct.Product) []string {
	if product == nil {
		return nil
	}
	urls := make([]string, 0, sheinProductImageURLCount(product))
	urls = appendSheinImageInfoURLs(urls, product.ImageInfo)
	for i := range product.SKCList {
		urls = appendSheinImageInfoURLs(urls, &product.SKCList[i].ImageInfo)
		for j := range product.SKCList[i].SKUS {
			urls = appendSheinImageInfoURLs(urls, product.SKCList[i].SKUS[j].ImageInfo)
		}
	}
	return urls
}

func appendSheinImageInfoURLs(urls []string, info *sheinproduct.ImageInfo) []string {
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

func buildSheinImageUploadPreflightSummary(report *SheinImageUploadPreflight) []string {
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
