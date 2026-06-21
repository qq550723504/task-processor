package listingkit

import (
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
	return sheinpub.UploadProductImages(product, uploader, cached, buildSheinColorBlockImageFromURL)
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
