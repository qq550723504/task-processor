package listingkit

import (
	"strings"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func applySheinStudioAIImagesToShein(pkg *sheinpub.Package, req *GenerateRequest) {
	if pkg == nil || req == nil || req.Options == nil || req.Options.SheinStudio == nil {
		return
	}
	productImages := uniqueNonEmptyStrings(req.Options.SheinStudio.ProductImageURLs)
	variantImages := normalizeSheinStudioVariantImageSets(req.Options.SheinStudio.VariantProductImages)
	sizeReferenceImages := uniqueNonEmptyStrings(req.Options.SheinStudio.SizeReferenceImageURLs)
	if len(productImages) == 0 {
		return
	}
	productImages = appendUniqueImageURLs(productImages, sizeReferenceImages...)

	sourceImages := uniqueNonEmptyStrings(append(
		append([]string(nil), req.Options.SheinStudio.SourceDesignURLs...),
		req.ImageURLs...,
	))
	if resolveSheinImageStrategy(req) == sheinImageStrategyHybrid {
		appendAIProductImagesToShein(pkg, productImages, sourceImages)
		applyVariantProductImagesToShein(pkg, variantImages, sourceImages)
		return
	}
	replaceSheinImagesWithAIProductImages(pkg, productImages, sourceImages)
	applyVariantProductImagesToShein(pkg, variantImages, sourceImages)
}

func replaceSheinImagesWithAIProductImages(pkg *sheinpub.Package, imageURLs []string, sourceImages []string) {
	images := imageSetFromAIProductImages(imageURLs, sourceImages)
	if images == nil {
		return
	}
	pkg.Images = images
	if pkg.RequestDraft != nil {
		pkg.RequestDraft.ImageInfo = sheinpub.BuildImageDraft(images)
		for skcIndex := range pkg.RequestDraft.SKCList {
			pkg.RequestDraft.SKCList[skcIndex].ImageInfo = sheinpub.BuildImageDraft(images)
			for skuIndex := range pkg.RequestDraft.SKCList[skcIndex].SKUList {
				pkg.RequestDraft.SKCList[skcIndex].SKUList[skuIndex].MainImage = images.MainImage
			}
		}
	}
	for skcIndex := range pkg.SkcList {
		pkg.SkcList[skcIndex].MainImageURL = images.MainImage
	}
	pkg.PreviewProduct = sheinpub.BuildPreviewProduct(pkg)
}

func appendAIProductImagesToShein(pkg *sheinpub.Package, imageURLs []string, sourceImages []string) {
	if len(imageURLs) == 0 {
		return
	}
	if pkg.Images == nil || strings.TrimSpace(pkg.Images.MainImage) == "" {
		replaceSheinImagesWithAIProductImages(pkg, imageURLs, sourceImages)
		return
	}
	pkg.Images.SourceImages = uniqueNonEmptyStrings(append(pkg.Images.SourceImages, sourceImages...))
	pkg.Images.Gallery = appendUniqueImageURLs(pkg.Images.Gallery, imageURLs...)
	if pkg.RequestDraft != nil {
		pkg.RequestDraft.ImageInfo = sheinpub.BuildImageDraft(pkg.Images)
		for skcIndex := range pkg.RequestDraft.SKCList {
			skcImages := imageDraftToSet(pkg.RequestDraft.SKCList[skcIndex].ImageInfo)
			if skcImages == nil || strings.TrimSpace(skcImages.MainImage) == "" {
				skcImages = pkg.Images
			} else {
				skcImages.SourceImages = uniqueNonEmptyStrings(append(skcImages.SourceImages, sourceImages...))
				skcImages.Gallery = appendUniqueImageURLs(skcImages.Gallery, imageURLs...)
			}
			pkg.RequestDraft.SKCList[skcIndex].ImageInfo = sheinpub.BuildImageDraft(skcImages)
		}
	}
	pkg.PreviewProduct = sheinpub.BuildPreviewProduct(pkg)
}

func imageSetFromAIProductImages(imageURLs []string, sourceImages []string) *common.ImageSet {
	imageURLs = uniqueNonEmptyStrings(imageURLs)
	if len(imageURLs) == 0 {
		return nil
	}
	images := &common.ImageSet{
		MainImage:    imageURLs[0],
		SourceImages: uniqueNonEmptyStrings(sourceImages),
	}
	if len(imageURLs) > 1 {
		images.Gallery = append([]string(nil), imageURLs[1:]...)
	}
	return images
}

func imageDraftToSet(draft *sheinpub.ImageDraft) *common.ImageSet {
	if draft == nil {
		return nil
	}
	return &common.ImageSet{
		MainImage:    draft.MainImage,
		Gallery:      append([]string(nil), draft.Gallery...),
		WhiteBgImage: draft.WhiteBg,
		SourceImages: append([]string(nil), draft.Source...),
	}
}

func appendUniqueImageURLs(existing []string, additions ...string) []string {
	result := append([]string(nil), existing...)
	seen := map[string]struct{}{}
	for _, imageURL := range result {
		imageURL = strings.TrimSpace(imageURL)
		if imageURL != "" {
			seen[imageURL] = struct{}{}
		}
	}
	for _, imageURL := range additions {
		imageURL = strings.TrimSpace(imageURL)
		if imageURL == "" {
			continue
		}
		if _, ok := seen[imageURL]; ok {
			continue
		}
		seen[imageURL] = struct{}{}
		result = append(result, imageURL)
	}
	return result
}

func normalizeSheinStudioVariantImageSets(input []SheinStudioVariantImageSet) []SheinStudioVariantImageSet {
	result := make([]SheinStudioVariantImageSet, 0, len(input))
	for _, item := range input {
		images := uniqueNonEmptyStrings(item.ImageURLs)
		if len(images) == 0 {
			continue
		}
		result = append(result, SheinStudioVariantImageSet{
			VariantSKU: strings.TrimSpace(item.VariantSKU),
			Color:      strings.TrimSpace(item.Color),
			ImageURLs:  images,
		})
	}
	return result
}

func applyVariantProductImagesToShein(pkg *sheinpub.Package, variantImages []SheinStudioVariantImageSet, sourceImages []string) {
	if pkg == nil || len(variantImages) == 0 {
		return
	}
	byColor := make(map[string]SheinStudioVariantImageSet, len(variantImages))
	bySKU := make(map[string]SheinStudioVariantImageSet, len(variantImages))
	for _, item := range variantImages {
		if key := normalizeVariantImageKey(item.Color); key != "" {
			byColor[key] = item
		}
		if key := normalizeVariantImageKey(item.VariantSKU); key != "" {
			bySKU[key] = item
		}
	}
	if pkg.RequestDraft != nil {
		for skcIndex := range pkg.RequestDraft.SKCList {
			skc := &pkg.RequestDraft.SKCList[skcIndex]
			if item, ok := findVariantImageSetForSKC(*skc, byColor, bySKU); ok {
				images := imageSetFromAIProductImages(item.ImageURLs, sourceImages)
				if images == nil {
					continue
				}
				skc.ImageInfo = sheinpub.BuildImageDraft(images)
				for skuIndex := range skc.SKUList {
					skc.SKUList[skuIndex].MainImage = images.MainImage
				}
			}
		}
	}
	for skcIndex := range pkg.SkcList {
		skc := &pkg.SkcList[skcIndex]
		if item, ok := findVariantImageSetForSKCPackage(*skc, byColor, bySKU); ok && len(item.ImageURLs) > 0 {
			skc.MainImageURL = item.ImageURLs[0]
		}
	}
	pkg.PreviewProduct = sheinpub.BuildPreviewProduct(pkg)
}

func findVariantImageSetForSKC(skc sheinpub.SKCRequestDraft, byColor map[string]SheinStudioVariantImageSet, bySKU map[string]SheinStudioVariantImageSet) (SheinStudioVariantImageSet, bool) {
	for _, candidate := range []string{
		skc.SaleName,
		skc.SkcName,
		saleAttributeValue(skc.SaleAttribute),
	} {
		if item, ok := byColor[normalizeVariantImageKey(candidate)]; ok {
			return item, true
		}
	}
	for _, sku := range skc.SKUList {
		if item, ok := bySKU[normalizeVariantImageKey(sku.Attributes["source_sds_sku"])]; ok {
			return item, true
		}
		if item, ok := byColor[normalizeVariantImageKey(sku.Attributes["Color"])]; ok {
			return item, true
		}
	}
	if item, ok := bySKU[normalizeVariantImageKey(strings.Split(skc.SupplierCode, "-")[0])]; ok {
		return item, true
	}
	return SheinStudioVariantImageSet{}, false
}

func findVariantImageSetForSKCPackage(skc sheinpub.SKCPackage, byColor map[string]SheinStudioVariantImageSet, bySKU map[string]SheinStudioVariantImageSet) (SheinStudioVariantImageSet, bool) {
	for _, candidate := range []string{skc.SaleName, skc.SkcName, skc.Attributes["Color"]} {
		if item, ok := byColor[normalizeVariantImageKey(candidate)]; ok {
			return item, true
		}
	}
	if item, ok := bySKU[normalizeVariantImageKey(strings.Split(skc.SupplierCode, "-")[0])]; ok {
		return item, true
	}
	return SheinStudioVariantImageSet{}, false
}

func saleAttributeValue(attr *sheinpub.ResolvedSaleAttribute) string {
	if attr == nil {
		return ""
	}
	return attr.Value
}

func normalizeVariantImageKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
