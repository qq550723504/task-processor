package listingkit

import (
	"encoding/json"
	"fmt"
	"strings"

	sheinimage "task-processor/internal/shein/api/image"
	sheinproduct "task-processor/internal/shein/api/product"
)

func cloneSheinProductForSubmit(product *sheinproduct.Product) (*sheinproduct.Product, error) {
	if product == nil {
		return nil, nil
	}
	data, err := json.Marshal(product)
	if err != nil {
		return nil, fmt.Errorf("clone shein product: %w", err)
	}
	var cloned sheinproduct.Product
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, fmt.Errorf("clone shein product: %w", err)
	}
	return &cloned, nil
}

func sheinProductImageURLCount(product *sheinproduct.Product) int {
	if product == nil {
		return 0
	}
	count := sheinImageInfoURLCount(product.ImageInfo)
	for i := range product.SKCList {
		count += sheinImageInfoURLCount(&product.SKCList[i].ImageInfo)
		for j := range product.SKCList[i].SKUS {
			count += sheinImageInfoURLCount(product.SKCList[i].SKUS[j].ImageInfo)
		}
	}
	return count
}

func sheinProductPendingImageUploadCount(product *sheinproduct.Product) int {
	if product == nil {
		return 0
	}
	count := sheinImageInfoPendingUploadCount(product.ImageInfo)
	for i := range product.SKCList {
		count += sheinImageInfoPendingUploadCount(&product.SKCList[i].ImageInfo)
		for j := range product.SKCList[i].SKUS {
			count += sheinImageInfoPendingUploadCount(product.SKCList[i].SKUS[j].ImageInfo)
		}
	}
	return count
}

func sheinImageInfoURLCount(info *sheinproduct.ImageInfo) int {
	if info == nil {
		return 0
	}
	count := 0
	for _, image := range info.ImageInfoList {
		if strings.TrimSpace(image.ImageURL) != "" {
			count++
		}
	}
	return count
}

func sheinImageInfoPendingUploadCount(info *sheinproduct.ImageInfo) int {
	if info == nil {
		return 0
	}
	count := 0
	for _, image := range info.ImageInfoList {
		url := strings.TrimSpace(image.ImageURL)
		if url != "" && !isSheinUploadedImageURL(url) {
			count++
		}
	}
	return count
}

func uploadSheinProductImages(product *sheinproduct.Product, uploader sheinimage.ImageAPI, cached map[string]string) (int, map[string]string, error) {
	if product == nil {
		return 0, cloneSheinImageUploadCache(cached), nil
	}
	if uploader == nil {
		return 0, cloneSheinImageUploadCache(cached), fmt.Errorf("shein image upload api is not configured")
	}
	uploaded := cloneSheinImageUploadCache(cached)
	count, err := uploadSheinImageInfo(product.ImageInfo, uploader, uploaded)
	if err != nil {
		return count, uploaded, err
	}
	for i := range product.SKCList {
		added, err := uploadSheinImageInfo(&product.SKCList[i].ImageInfo, uploader, uploaded)
		count += added
		if err != nil {
			return count, uploaded, err
		}
		for j := range product.SKCList[i].SKUS {
			added, err := uploadSheinImageInfo(product.SKCList[i].SKUS[j].ImageInfo, uploader, uploaded)
			count += added
			if err != nil {
				return count, uploaded, err
			}
		}
	}
	return count, uploaded, nil
}

func buildSheinImageUploadPreflight(pkg *SheinPackage) *SheinImageUploadPreflight {
	if pkg == nil || pkg.PreviewProduct == nil {
		return nil
	}
	urls := collectSheinProductImageURLs(pkg.PreviewProduct)
	if len(urls) == 0 {
		return &SheinImageUploadPreflight{
			ReadyForUpload: false,
			Summary:        []string{"SHEIN preview_product has no image_info URLs to submit."},
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

func isSheinUploadedImageURL(url string) bool {
	value := strings.ToLower(strings.TrimSpace(url))
	return strings.Contains(value, "shein.com") ||
		strings.Contains(value, "sheinimg.com") ||
		strings.Contains(value, "ltwebstatic.com")
}

func isSDSImageURL(url string) bool {
	value := strings.ToLower(strings.TrimSpace(url))
	return strings.Contains(value, "sdspod.com") || strings.Contains(value, "sdsdiy.com")
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

func uploadSheinImageInfo(info *sheinproduct.ImageInfo, uploader sheinimage.ImageAPI, uploaded map[string]string) (int, error) {
	if info == nil {
		return 0, nil
	}
	count := 0
	for i := range info.ImageInfoList {
		sourceURL := strings.TrimSpace(info.ImageInfoList[i].ImageURL)
		if sourceURL == "" {
			continue
		}
		isColorBlock := info.ImageInfoList[i].ImageType == 6 && !info.ImageInfoList[i].SizeImgFlag
		if isSheinUploadedImageURL(sourceURL) {
			continue
		}
		cacheKey := sourceURL
		if isColorBlock {
			cacheKey = "color-block:" + sourceURL
		}
		uploadedURL, ok := uploaded[cacheKey]
		if ok && !isSheinUploadedImageURL(uploadedURL) {
			ok = false
		}
		if !ok {
			var err error
			if isColorBlock {
				var imageData []byte
				imageData, err = buildSheinColorBlockImageFromURL(sourceURL)
				if err == nil {
					uploadedURL, err = uploader.UploadOriginalImage(imageData)
				}
				if err != nil {
					if existingURL := strings.TrimSpace(uploaded[sourceURL]); isSheinUploadedImageURL(existingURL) {
						uploadedURL = existingURL
						err = nil
					} else {
						uploadedURL, err = uploader.DownloadAndUploadImage(sourceURL)
					}
				}
			} else {
				uploadedURL, err = uploader.DownloadAndUploadImage(sourceURL)
			}
			if err != nil {
				return count, fmt.Errorf("upload shein image %q: %w", sourceURL, err)
			}
			uploaded[cacheKey] = uploadedURL
			count++
		}
		info.ImageInfoList[i].ImageURL = uploadedURL
	}
	return count, nil
}

func sheinImageUploadCache(pkg *SheinPackage) map[string]string {
	if pkg == nil || pkg.FinalDraft == nil {
		return nil
	}
	return pkg.FinalDraft.SheinImageUploadCache
}

func sheinImageUploadCacheHit(pkg *SheinPackage, sourceURL string) bool {
	uploadedURL := strings.TrimSpace(sheinImageUploadCache(pkg)[strings.TrimSpace(sourceURL)])
	return uploadedURL != "" && isSheinUploadedImageURL(uploadedURL)
}

func cloneSheinImageUploadCache(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	for sourceURL, uploadedURL := range input {
		sourceURL = strings.TrimSpace(sourceURL)
		uploadedURL = strings.TrimSpace(uploadedURL)
		if sourceURL == "" || uploadedURL == "" || !isSheinUploadedImageURL(uploadedURL) {
			continue
		}
		out[sourceURL] = uploadedURL
	}
	return out
}
