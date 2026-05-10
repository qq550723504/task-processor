package listingkit

import sheinpub "task-processor/internal/publishing/shein"

func applySheinStudioAIImagesToShein(pkg *sheinpub.Package, req *GenerateRequest, sdsSummary *SDSSyncSummary) {
	if pkg == nil || req == nil || req.Options == nil || req.Options.SheinStudio == nil {
		return
	}
	productImages := uniqueNonEmptyStrings(req.Options.SheinStudio.ProductImageURLs)
	variantImages := normalizeSheinStudioVariantImageSets(req.Options.SheinStudio.VariantProductImages)
	sizeReferenceImages := resolveSheinSizeReferenceImages(req, sdsSummary)
	sourceImages := uniqueNonEmptyStrings(append(
		append([]string(nil), req.Options.SheinStudio.SourceDesignURLs...),
		req.ImageURLs...,
	))
	if len(productImages) == 0 {
		if sdsSummary != nil && len(sdsSummary.MockupImageURLs) > 0 {
			applySDSTemplateImagesToShein(pkg, sdsSummary, sourceImages)
			applySheinSizeReferenceImages(pkg, sizeReferenceImages)
		}
		return
	}
	productImages = appendUniqueImageURLs(productImages, sizeReferenceImages...)
	if resolveSheinImageStrategy(req) == sheinImageStrategyHybrid {
		appendAIProductImagesToShein(pkg, productImages, sourceImages)
		applyVariantProductImagesToShein(pkg, variantImages, sourceImages)
		applySheinSizeReferenceImages(pkg, sizeReferenceImages)
		return
	}
	replaceSheinImagesWithAIProductImages(pkg, productImages, sourceImages)
	applyVariantProductImagesToShein(pkg, variantImages, sourceImages)
	applySheinSizeReferenceImages(pkg, sizeReferenceImages)
}
