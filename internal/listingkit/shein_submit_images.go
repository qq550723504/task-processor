package listingkit

import (
	"encoding/json"
	"fmt"
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
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
	refs := collectSheinProductImageRefs(product)
	pending := map[string]sheinImageUploadJob{}
	for _, ref := range refs {
		uploadedURL, ok := uploaded[ref.cacheKey]
		if ok && !isSheinUploadedImageURL(uploadedURL) {
			ok = false
		}
		if ok {
			continue
		}
		if _, exists := pending[ref.cacheKey]; exists {
			continue
		}
		pending[ref.cacheKey] = sheinImageUploadJob{
			cacheKey:     ref.cacheKey,
			sourceURL:    ref.sourceURL,
			isColorBlock: ref.isColorBlock,
		}
	}
	count, err := uploadSheinImageJobs(pending, uploader, uploaded)
	if err != nil {
		return count, uploaded, err
	}
	for _, ref := range refs {
		if uploadedURL := strings.TrimSpace(uploaded[ref.cacheKey]); isSheinUploadedImageURL(uploadedURL) {
			ref.image.ImageURL = uploadedURL
		}
	}
	return count, uploaded, nil
}

const sheinSubmitImageUploadConcurrency = 3

type sheinImageUploadRef struct {
	image        *sheinproduct.ImageDetail
	sourceURL    string
	cacheKey     string
	isColorBlock bool
}

type sheinImageUploadJob struct {
	cacheKey     string
	sourceURL    string
	isColorBlock bool
}

type sheinImageUploadResult struct {
	cacheKey    string
	uploadedURL string
	err         error
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

func sheinImageUploadCache(pkg *SheinPackage) map[string]string {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.FinalSubmissionDraft == nil {
		return nil
	}
	return pkg.FinalSubmissionDraft.SheinImageUploadCache
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
