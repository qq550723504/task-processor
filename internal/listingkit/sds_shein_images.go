package listingkit

import (
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func applySelectedSDSImagesToShein(pkg *sheinpub.Package, req *GenerateRequest, sourceImages []string) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || req == nil || req.Options == nil || req.Options.SheinStudio == nil {
		return false
	}
	selected := normalizeSelectedSDSImages(req.Options.SheinStudio.SelectedSDSImages)
	if len(selected) == 0 {
		return false
	}

	defaultImages := imageSetFromSelectedSDSImages(selected, sourceImages)
	if defaultImages == nil {
		return false
	}

	byColor := map[string]*common.ImageSet{}
	bySKU := map[string]*common.ImageSet{}
	for _, item := range selected {
		images := &common.ImageSet{
			MainImage:    item.ImageURL,
			SourceImages: uniqueNonEmptyStrings(sourceImages),
		}
		if sku := sheinpub.NormalizeSDSImageKey(item.VariantSKU); sku != "__default__" {
			bySKU[sku] = sheinpub.MergeSDSImageSet(bySKU[sku], images)
		}
		if color := sheinpub.NormalizeSDSImageKey(item.Color); color != "__default__" {
			byColor[color] = sheinpub.MergeSDSImageSet(byColor[color], images)
		}
	}

	pkg.Images = defaultImages
	if pkg.DraftPayload != nil {
		pkg.DraftPayload.ImageInfo = sheinpub.BuildImageDraft(defaultImages)
		for skcIndex := range pkg.DraftPayload.SKCList {
			skcImages := sheinpub.ResolveSDSImagesForSKC(pkg, skcIndex, bySKU, byColor)
			if skcImages == nil {
				continue
			}
			pkg.DraftPayload.SKCList[skcIndex].ImageInfo = sheinpub.BuildImageDraft(skcImages)
			for skuIndex := range pkg.DraftPayload.SKCList[skcIndex].SKUList {
				skuImages := sheinpub.ResolveSDSImagesForSKU(&pkg.DraftPayload.SKCList[skcIndex].SKUList[skuIndex], bySKU, byColor)
				if skuImages == nil {
					skuImages = skcImages
				}
				pkg.DraftPayload.SKCList[skcIndex].SKUList[skuIndex].MainImage = skuImages.MainImage
			}
		}
	}
	for skcIndex := range pkg.SkcList {
		skcImages := sheinpub.ResolveSDSImagesForSKC(pkg, skcIndex, bySKU, byColor)
		if skcImages == nil {
			skcImages = defaultImages
		}
		pkg.SkcList[skcIndex].MainImageURL = skcImages.MainImage
	}
	preview := sheinpub.BuildPreviewProduct(pkg)
	sheinpub.SetPreviewPayload(pkg, preview)
	return true
}

func applySDSOfficialImagesToShein(pkg *sheinpub.Package, _ *GenerateRequest, summary *SDSSyncSummary, options *SDSSyncOptions) bool {
	return applySDSTemplateImagesToSheinWithResult(pkg, summary, nil, options)
}

func applySDSTemplateImagesToShein(pkg *sheinpub.Package, summary *SDSSyncSummary, sourceImages []string, options ...*SDSSyncOptions) {
	var sdsOptions *SDSSyncOptions
	if len(options) > 0 {
		sdsOptions = options[0]
	}
	_ = applySDSTemplateImagesToSheinWithResult(pkg, summary, sourceImages, sdsOptions)
}

func applySDSTemplateImagesToSheinWithResult(pkg *sheinpub.Package, summary *SDSSyncSummary, sourceImages []string, options *SDSSyncOptions) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || summary == nil {
		return false
	}
	if len(summary.VariantResults) > 0 {
		return applySDSVariantTemplateImagesToShein(pkg, summary, sourceImages, options)
	}
	if summary.Status == "failed" {
		return false
	}
	if len(summary.MockupImageURLs) == 0 {
		return false
	}

	images := sheinpub.ImageSetFromSDSMockups(summary.MockupImageURLs, sourceImages)
	if images == nil {
		return false
	}
	pkg.Images = images

	if pkg.DraftPayload != nil {
		pkg.DraftPayload.ImageInfo = sheinpub.BuildImageDraft(images)
		for skcIndex := range pkg.DraftPayload.SKCList {
			pkg.DraftPayload.SKCList[skcIndex].ImageInfo = sheinpub.BuildImageDraft(images)
			for skuIndex := range pkg.DraftPayload.SKCList[skcIndex].SKUList {
				pkg.DraftPayload.SKCList[skcIndex].SKUList[skuIndex].MainImage = images.MainImage
			}
		}
	}
	for skcIndex := range pkg.SkcList {
		pkg.SkcList[skcIndex].MainImageURL = images.MainImage
	}
	preview := sheinpub.BuildPreviewProduct(pkg)
	sheinpub.SetPreviewPayload(pkg, preview)
	return true
}

func applySDSVariantTemplateImagesToShein(pkg *sheinpub.Package, summary *SDSSyncSummary, sourceImages []string, options *SDSSyncOptions) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	byColor := map[string]*common.ImageSet{}
	bySKU := map[string]*common.ImageSet{}
	for i := range summary.VariantResults {
		item := &summary.VariantResults[i]
		if len(item.MockupImageURLs) == 0 || item.Status == "failed" {
			continue
		}
		images := sheinpub.ImageSetFromSDSMockups(item.MockupImageURLs, sourceImages)
		sheinpub.RegisterSDSVariantImageSet(bySKU, byColor, item.VariantSKU, item.VariantColor, images, true)
	}
	if len(byColor) == 0 && len(bySKU) == 0 {
		return false
	}

	defaultImages := byColor[sheinpub.NormalizeSDSImageKey(summary.VariantColor)]
	if defaultImages == nil {
		for _, item := range summary.VariantResults {
			if images := byColor[sheinpub.NormalizeSDSImageKey(item.VariantColor)]; images != nil {
				defaultImages = images
				break
			}
			if images := bySKU[sheinpub.NormalizeSDSImageKey(item.VariantSKU)]; images != nil {
				defaultImages = images
				break
			}
		}
	}
	if defaultImages == nil && options != nil {
		for _, item := range options.Variants {
			if images := byColor[sheinpub.NormalizeSDSImageKey(item.Color)]; images != nil {
				defaultImages = images
				break
			}
			if images := bySKU[sheinpub.NormalizeSDSImageKey(item.VariantSKU)]; images != nil {
				defaultImages = images
				break
			}
		}
	}
	if defaultImages == nil {
		defaultImages = sheinpub.FirstSDSImageSet(byColor)
	}
	if defaultImages == nil {
		defaultImages = sheinpub.FirstSDSImageSet(bySKU)
	}
	if defaultImages == nil {
		return false
	}

	pkg.Images = defaultImages
	if pkg.DraftPayload != nil {
		pkg.DraftPayload.ImageInfo = sheinpub.BuildImageDraft(defaultImages)
		for skcIndex := range pkg.DraftPayload.SKCList {
			skcImages := sheinpub.ResolveSDSImagesForSKC(pkg, skcIndex, bySKU, byColor)
			if skcImages == nil {
				continue
			}
			pkg.DraftPayload.SKCList[skcIndex].ImageInfo = sheinpub.BuildImageDraft(skcImages)
			for skuIndex := range pkg.DraftPayload.SKCList[skcIndex].SKUList {
				skuImages := sheinpub.ResolveSDSImagesForSKU(&pkg.DraftPayload.SKCList[skcIndex].SKUList[skuIndex], bySKU, byColor)
				if skuImages == nil {
					skuImages = skcImages
				}
				pkg.DraftPayload.SKCList[skcIndex].SKUList[skuIndex].MainImage = skuImages.MainImage
			}
		}
	}
	for skcIndex := range pkg.SkcList {
		images := sheinpub.ResolveSDSImagesForSKC(pkg, skcIndex, bySKU, byColor)
		if images == nil {
			continue
		}
		pkg.SkcList[skcIndex].MainImageURL = images.MainImage
	}
	preview := sheinpub.BuildPreviewProduct(pkg)
	sheinpub.SetPreviewPayload(pkg, preview)
	return true
}

func hasSDSVariantOptionMockups(options *SDSSyncOptions) bool {
	if options == nil {
		return false
	}
	for _, item := range options.Variants {
		if imageSetFromSDSVariantOption(item, nil) != nil {
			return true
		}
	}
	return false
}
