package listingkit

import (
	"fmt"
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
	sheinimage "task-processor/internal/shein/api/image"
	sheinproduct "task-processor/internal/shein/api/product"
)

func cloneSheinProductForSubmit(product *sheinproduct.Product) (*sheinproduct.Product, error) {
	return sheinpub.CloneProductForSubmit(product)
}

func sheinProductImageURLCount(product *sheinproduct.Product) int {
	return sheinpub.ProductImageURLCount(product)
}

func sheinProductPendingImageUploadCount(product *sheinproduct.Product) int {
	return sheinpub.ProductPendingImageUploadCount(product)
}

func sheinImageInfoURLCount(info *sheinproduct.ImageInfo) int {
	return sheinpub.ImageInfoURLCount(info)
}

func sheinImageInfoPendingUploadCount(info *sheinproduct.ImageInfo) int {
	return sheinpub.ImageInfoPendingUploadCount(info)
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
	return sheinpub.IsUploadedImageURL(url)
}

func isSDSImageURL(url string) bool {
	return sheinpub.IsSDSImageURL(url)
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
	return sheinpub.CloneImageUploadCache(input)
}
