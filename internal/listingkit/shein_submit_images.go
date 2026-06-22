package listingkit

import (
	sheinpub "task-processor/internal/publishing/shein"
	sheinimage "task-processor/internal/shein/api/image"
	sheinproduct "task-processor/internal/shein/api/product"
)

func uploadSheinProductImages(product *sheinproduct.Product, uploader sheinimage.ImageAPI, cached map[string]string) (int, map[string]string, error) {
	return sheinpub.UploadProductImages(product, uploader, cached, buildSheinColorBlockImageFromURL)
}

func sheinImageUploadCache(pkg *SheinPackage) map[string]string {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.FinalSubmissionDraft == nil {
		return nil
	}
	return pkg.FinalSubmissionDraft.SheinImageUploadCache
}
