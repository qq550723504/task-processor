package listingkit

import (
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func normalizeSheinSubmitImages(product *sheinproduct.Product) {
	sheinpub.NormalizeSubmitImages(product)
}

func normalizeSheinSubmitSPUImages(images []sheinproduct.ImageDetail) []sheinproduct.ImageDetail {
	return sheinpub.NormalizeSubmitSPUImages(images)
}

func normalizeSheinSubmitSKUImages(skc *sheinproduct.SKC) {
	sheinpub.NormalizeSubmitSKUImages(skc)
}

func normalizeSheinSubmitSKUImageDetail(image sheinproduct.ImageDetail) sheinproduct.ImageDetail {
	return sheinpub.NormalizeSubmitSKUImageDetail(image)
}

func normalizeSheinSubmitSKCImages(skc *sheinproduct.SKC) {
	sheinpub.NormalizeSubmitSKCImages(skc)
}

func buildSheinSubmitSiteDetailImageInfoList(images []sheinproduct.ImageDetail) []sheinproduct.SiteDetailImageInfo {
	return sheinpub.BuildSubmitSiteDetailImageInfoList(images)
}

func normalizeSheinSubmitGalleryImages(images []sheinproduct.ImageDetail, includeColorBlock bool) []sheinproduct.ImageDetail {
	return sheinpub.NormalizeSubmitGalleryImages(images, includeColorBlock)
}

func dedupeSheinImagesByURL(images []sheinproduct.ImageDetail) []sheinproduct.ImageDetail {
	return sheinpub.DedupeImagesByURL(images)
}
