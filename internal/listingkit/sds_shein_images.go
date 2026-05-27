package listingkit

import (
	"strings"

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
		if sku := normalizeSDSColorKey(item.VariantSKU); sku != "__default__" {
			bySKU[sku] = mergeImageSet(bySKU[sku], images)
		}
		if color := normalizeSDSColorKey(item.Color); color != "__default__" {
			byColor[color] = mergeImageSet(byColor[color], images)
		}
	}

	pkg.Images = defaultImages
	if pkg.DraftPayload != nil {
		pkg.DraftPayload.ImageInfo = sheinpub.BuildImageDraft(defaultImages)
		for skcIndex := range pkg.DraftPayload.SKCList {
			skcImages := resolveSDSImagesForSKC(pkg, skcIndex, bySKU, byColor)
			if skcImages == nil {
				continue
			}
			pkg.DraftPayload.SKCList[skcIndex].ImageInfo = sheinpub.BuildImageDraft(skcImages)
			for skuIndex := range pkg.DraftPayload.SKCList[skcIndex].SKUList {
				skuImages := resolveSDSImagesForSKU(&pkg.DraftPayload.SKCList[skcIndex].SKUList[skuIndex], bySKU, byColor)
				if skuImages == nil {
					skuImages = skcImages
				}
				pkg.DraftPayload.SKCList[skcIndex].SKUList[skuIndex].MainImage = skuImages.MainImage
			}
		}
	}
	for skcIndex := range pkg.SkcList {
		skcImages := resolveSDSImagesForSKC(pkg, skcIndex, bySKU, byColor)
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

	images := imageSetFromSDSMockups(summary.MockupImageURLs, sourceImages)
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
		images := imageSetFromSDSMockups(item.MockupImageURLs, sourceImages)
		registerSDSVariantImageSet(bySKU, byColor, item.VariantSKU, item.VariantColor, images, true)
	}
	if len(byColor) == 0 && len(bySKU) == 0 {
		return false
	}

	defaultImages := byColor[normalizeSDSColorKey(summary.VariantColor)]
	if defaultImages == nil {
		for _, item := range summary.VariantResults {
			if images := byColor[normalizeSDSColorKey(item.VariantColor)]; images != nil {
				defaultImages = images
				break
			}
			if images := bySKU[normalizeSDSColorKey(item.VariantSKU)]; images != nil {
				defaultImages = images
				break
			}
		}
	}
	if defaultImages == nil && options != nil {
		for _, item := range options.Variants {
			if images := byColor[normalizeSDSColorKey(item.Color)]; images != nil {
				defaultImages = images
				break
			}
			if images := bySKU[normalizeSDSColorKey(item.VariantSKU)]; images != nil {
				defaultImages = images
				break
			}
		}
	}
	if defaultImages == nil {
		defaultImages = firstSDSImageSet(byColor)
	}
	if defaultImages == nil {
		defaultImages = firstSDSImageSet(bySKU)
	}
	if defaultImages == nil {
		return false
	}

	pkg.Images = defaultImages
	if pkg.DraftPayload != nil {
		pkg.DraftPayload.ImageInfo = sheinpub.BuildImageDraft(defaultImages)
		for skcIndex := range pkg.DraftPayload.SKCList {
			skcImages := resolveSDSImagesForSKC(pkg, skcIndex, bySKU, byColor)
			if skcImages == nil {
				continue
			}
			pkg.DraftPayload.SKCList[skcIndex].ImageInfo = sheinpub.BuildImageDraft(skcImages)
			for skuIndex := range pkg.DraftPayload.SKCList[skcIndex].SKUList {
				skuImages := resolveSDSImagesForSKU(&pkg.DraftPayload.SKCList[skcIndex].SKUList[skuIndex], bySKU, byColor)
				if skuImages == nil {
					skuImages = skcImages
				}
				pkg.DraftPayload.SKCList[skcIndex].SKUList[skuIndex].MainImage = skuImages.MainImage
			}
		}
	}
	for skcIndex := range pkg.SkcList {
		images := resolveSDSImagesForSKC(pkg, skcIndex, bySKU, byColor)
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

func imageSetFromSDSVariantOption(item SDSSyncVariantOption, sourceImages []string) *common.ImageSet {
	mockups := uniqueNonEmptyStrings(item.MockupImageURLs)
	if len(mockups) == 0 {
		mockups = uniqueNonEmptyStrings([]string{item.MockupImageURL})
	}
	if len(mockups) == 0 {
		return nil
	}
	return imageSetFromSDSMockups(mockups, sourceImages)
}

func registerSDSVariantImageSet(bySKU map[string]*common.ImageSet, byColor map[string]*common.ImageSet, sku string, color string, images *common.ImageSet, overwrite bool) {
	if images == nil {
		return
	}
	if key := normalizeSDSColorKey(color); key != "__default__" {
		if overwrite || byColor[key] == nil {
			byColor[key] = images
		}
	}
	if key := normalizeSDSColorKey(sku); key != "__default__" {
		if overwrite || bySKU[key] == nil {
			bySKU[key] = images
		}
	}
}

func firstSDSImageSet(values map[string]*common.ImageSet) *common.ImageSet {
	for _, images := range values {
		if images != nil {
			return images
		}
	}
	return nil
}

func resolveSDSImagesForSKC(pkg *sheinpub.Package, index int, bySKU map[string]*common.ImageSet, byColor map[string]*common.ImageSet) *common.ImageSet {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || index < 0 {
		return nil
	}
	if index < len(pkg.DraftPayload.SKCList) {
		skc := &pkg.DraftPayload.SKCList[index]
		for _, value := range sdsSKUCandidatesFromRequestSKC(skc) {
			if images := lookupSDSImagesBySKU(bySKU, value); images != nil {
				return images
			}
		}
		for _, value := range []string{
			skcSaleAttributeValue(skc.SaleAttribute),
			skcColorFromDraft(skc),
		} {
			if images := byColor[normalizeSDSColorKey(value)]; images != nil {
				return images
			}
		}
	}
	if index < len(pkg.SkcList) {
		skc := &pkg.SkcList[index]
		for _, value := range sdsSKUCandidatesFromPackageSKC(skc) {
			if images := lookupSDSImagesBySKU(bySKU, value); images != nil {
				return images
			}
		}
		attrs := skc.Attributes
		for _, value := range []string{
			attrs["Color"],
			attrs["color"],
			skc.SaleName,
			skc.SkcName,
		} {
			if images := byColor[normalizeSDSColorKey(value)]; images != nil {
				return images
			}
		}
	}
	return nil
}

func lookupSDSImagesBySKU(bySKU map[string]*common.ImageSet, value string) *common.ImageSet {
	if images := bySKU[normalizeSDSColorKey(value)]; images != nil {
		return images
	}
	return nil
}

func sdsSKUCandidatesFromRequestSKC(skc *sheinpub.SKCRequestDraft) []string {
	if skc == nil {
		return nil
	}
	values := []string{
		sourceSDSSKUFromSupplierSKU(skc.SupplierCode),
		skc.SupplierCode,
	}
	for _, sku := range skc.SKUList {
		values = append(values,
			sku.Attributes["source_sds_sku"],
			sourceSDSSKUFromSupplierSKU(sku.SupplierSKU),
			sku.SupplierSKU,
		)
	}
	return values
}

func sdsSKUCandidatesFromPackageSKC(skc *sheinpub.SKCPackage) []string {
	if skc == nil {
		return nil
	}
	values := []string{
		sourceSDSSKUFromSupplierSKU(skc.SupplierCode),
		skc.SupplierCode,
	}
	for _, sku := range skc.SKUs {
		values = append(values,
			sku.Attributes["source_sds_sku"],
			sourceSDSSKUFromSupplierSKU(sku.SKU),
			sku.SKU,
		)
	}
	return values
}

func resolveSDSImagesForSKU(sku *sheinpub.SKUDraft, bySKU map[string]*common.ImageSet, byColor map[string]*common.ImageSet) *common.ImageSet {
	if sku == nil {
		return nil
	}
	if images := bySKU[normalizeSDSColorKey(sourceSDSSKUFromSupplierSKU(sku.SupplierSKU))]; images != nil {
		return images
	}
	if images := bySKU[normalizeSDSColorKey(sku.Attributes["source_sds_sku"])]; images != nil {
		return images
	}
	if images := byColor[normalizeSDSColorKey(sku.Attributes["Color"])]; images != nil {
		return images
	}
	if images := byColor[normalizeSDSColorKey(sku.Attributes["color"])]; images != nil {
		return images
	}
	return nil
}

func sourceSDSSKUFromSupplierSKU(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if index := strings.LastIndex(value, "-"); index > 0 {
		return strings.TrimSpace(value[:index])
	}
	return value
}

func imageSetFromSDSMockups(mockups []string, sourceImages []string) *common.ImageSet {
	mockups = uniqueNonEmptyStrings(mockups)
	if len(mockups) == 0 {
		return nil
	}
	images := &common.ImageSet{
		MainImage:    mockups[0],
		SourceImages: uniqueNonEmptyStrings(sourceImages),
	}
	if len(mockups) > 1 {
		images.Gallery = append([]string(nil), mockups[1:]...)
	}
	return images
}

func imageSetFromSelectedSDSImages(items []SheinStudioSelectedSDSImage, sourceImages []string) *common.ImageSet {
	if len(items) == 0 {
		return nil
	}
	images := &common.ImageSet{
		MainImage:    items[0].ImageURL,
		SourceImages: uniqueNonEmptyStrings(sourceImages),
	}
	for _, item := range items[1:] {
		if imageURL := strings.TrimSpace(item.ImageURL); imageURL != "" {
			images.Gallery = append(images.Gallery, imageURL)
		}
	}
	return images
}

func normalizeSelectedSDSImages(input []SheinStudioSelectedSDSImage) []SheinStudioSelectedSDSImage {
	result := make([]SheinStudioSelectedSDSImage, 0, len(input))
	seen := map[string]struct{}{}
	for _, item := range input {
		imageURL := strings.TrimSpace(item.ImageURL)
		if imageURL == "" {
			continue
		}
		if _, ok := seen[imageURL]; ok {
			continue
		}
		seen[imageURL] = struct{}{}
		result = append(result, SheinStudioSelectedSDSImage{
			ImageURL:   imageURL,
			VariantSKU: strings.TrimSpace(item.VariantSKU),
			Color:      strings.TrimSpace(item.Color),
		})
	}
	return result
}

func mergeImageSet(existing *common.ImageSet, next *common.ImageSet) *common.ImageSet {
	if next == nil || strings.TrimSpace(next.MainImage) == "" {
		return existing
	}
	if existing == nil || strings.TrimSpace(existing.MainImage) == "" {
		return &common.ImageSet{
			MainImage:    next.MainImage,
			Gallery:      append([]string(nil), next.Gallery...),
			SourceImages: append([]string(nil), next.SourceImages...),
		}
	}
	existing.Gallery = appendUniqueImageURLs(existing.Gallery, next.MainImage)
	existing.Gallery = appendUniqueImageURLs(existing.Gallery, next.Gallery...)
	return existing
}

func skcSaleAttributeValue(attribute *sheinpub.ResolvedSaleAttribute) string {
	if attribute == nil {
		return ""
	}
	return attribute.Value
}

func skcColorFromDraft(skc *sheinpub.SKCRequestDraft) string {
	if skc == nil {
		return ""
	}
	for _, sku := range skc.SKUList {
		if value := strings.TrimSpace(sku.Attributes["Color"]); value != "" {
			return value
		}
		if value := strings.TrimSpace(sku.Attributes["color"]); value != "" {
			return value
		}
	}
	return ""
}

func normalizeSDSColorKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "__default__"
	}
	return value
}
