package listingkit

import (
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func sheinHasAnySKU(pkg *SheinPackage) bool {
	return sheinpub.HasAnySubmitSKU(pkg)
}

func sheinFinalImagesReady(pkg *SheinPackage) (bool, string) {
	return sheinFinalImagesReadyForAction(pkg, "publish")
}

func sheinFinalImagesReadyForAction(pkg *SheinPackage, action string) (bool, string) {
	return sheinpub.FinalSubmitImagesReady(pkg, action)
}

func sheinHasFinalGalleryImage(pkg *SheinPackage) bool {
	return sheinpub.HasFinalGalleryImage(pkg)
}

func sheinHasSKCImage(pkg *SheinPackage) bool {
	return sheinpub.HasSKCImage(pkg)
}

func sheinHasSwatchRole(pkg *SheinPackage) bool {
	return sheinpub.HasSwatchRole(pkg)
}

func sheinHasSingleSKC(pkg *SheinPackage) bool {
	return sheinpub.HasSingleSKC(pkg)
}

func sheinHasFinalMainImage(pkg *SheinPackage) bool {
	return sheinpub.HasFinalMainImage(pkg)
}

func sheinPricingReady(pkg *SheinPackage) bool {
	return sheinpub.SubmitPricingReady(pkg)
}

func sheinHasSubmitImage(pkg *SheinPackage) bool {
	return sheinpub.HasSubmitImage(pkg)
}

func sheinImageDraftHasImage(info *SheinImageDraft) bool {
	return sheinpub.ImageDraftHasImage(info)
}

func sheinProductImageInfoHasImage(info *sheinproduct.ImageInfo) bool {
	return sheinpub.ProductImageInfoHasImage(info)
}
