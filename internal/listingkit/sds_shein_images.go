package listingkit

import (
	"strings"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func applySelectedSDSImagesToShein(pkg *sheinpub.Package, req *GenerateRequest, sourceImages []string) bool {
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
	if pkg.RequestDraft != nil {
		pkg.RequestDraft.ImageInfo = sheinpub.BuildImageDraft(defaultImages)
		for skcIndex := range pkg.RequestDraft.SKCList {
			skcImages := resolveSDSImagesForSKC(pkg, skcIndex, bySKU, byColor)
			if skcImages == nil {
				skcImages = defaultImages
			}
			pkg.RequestDraft.SKCList[skcIndex].ImageInfo = sheinpub.BuildImageDraft(skcImages)
			for skuIndex := range pkg.RequestDraft.SKCList[skcIndex].SKUList {
				skuImages := resolveSDSImagesForSKU(&pkg.RequestDraft.SKCList[skcIndex].SKUList[skuIndex], bySKU, byColor)
				if skuImages == nil {
					skuImages = skcImages
				}
				pkg.RequestDraft.SKCList[skcIndex].SKUList[skuIndex].MainImage = skuImages.MainImage
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
	pkg.PreviewProduct = sheinpub.BuildPreviewProduct(pkg)
	return true
}

func applySDSTemplateImagesToShein(pkg *sheinpub.Package, summary *SDSSyncSummary, sourceImages []string) {
	if pkg == nil || summary == nil {
		return
	}
	if summary.Status == "failed" {
		return
	}
	if len(summary.VariantResults) > 0 {
		applySDSVariantTemplateImagesToShein(pkg, summary, sourceImages)
		return
	}
	if len(summary.MockupImageURLs) == 0 {
		return
	}

	images := &common.ImageSet{
		MainImage:    summary.MockupImageURLs[0],
		SourceImages: uniqueNonEmptyStrings(sourceImages),
	}
	if len(summary.MockupImageURLs) > 1 {
		images.Gallery = append([]string(nil), summary.MockupImageURLs[1:]...)
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

func applySDSVariantTemplateImagesToShein(pkg *sheinpub.Package, summary *SDSSyncSummary, sourceImages []string) {
	byColor := map[string]*common.ImageSet{}
	bySKU := map[string]*common.ImageSet{}
	for i := range summary.VariantResults {
		item := &summary.VariantResults[i]
		if len(item.MockupImageURLs) == 0 || item.Status == "failed" {
			continue
		}
		images := imageSetFromSDSMockups(item.MockupImageURLs, sourceImages)
		key := normalizeSDSColorKey(item.VariantColor)
		if _, exists := byColor[key]; !exists {
			byColor[key] = images
		}
		if sku := normalizeSDSColorKey(item.VariantSKU); sku != "__default__" {
			bySKU[sku] = images
		}
	}
	if len(byColor) == 0 {
		return
	}

	defaultImages := byColor[normalizeSDSColorKey(summary.VariantColor)]
	if defaultImages == nil {
		for _, item := range summary.VariantResults {
			if images := byColor[normalizeSDSColorKey(item.VariantColor)]; images != nil {
				defaultImages = images
				break
			}
		}
	}
	if defaultImages == nil {
		return
	}
	pkg.Images = defaultImages
	if pkg.RequestDraft != nil {
		pkg.RequestDraft.ImageInfo = sheinpub.BuildImageDraft(defaultImages)
		for skcIndex := range pkg.RequestDraft.SKCList {
			skcImages := resolveSDSImagesForSKC(pkg, skcIndex, bySKU, byColor)
			if skcImages == nil {
				skcImages = defaultImages
			}
			pkg.RequestDraft.SKCList[skcIndex].ImageInfo = sheinpub.BuildImageDraft(skcImages)
			for skuIndex := range pkg.RequestDraft.SKCList[skcIndex].SKUList {
				skuImages := resolveSDSImagesForSKU(&pkg.RequestDraft.SKCList[skcIndex].SKUList[skuIndex], bySKU, byColor)
				if skuImages == nil {
					skuImages = skcImages
				}
				pkg.RequestDraft.SKCList[skcIndex].SKUList[skuIndex].MainImage = skuImages.MainImage
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
	pkg.PreviewProduct = sheinpub.BuildPreviewProduct(pkg)
}

func resolveSDSImagesForSKC(pkg *sheinpub.Package, index int, bySKU map[string]*common.ImageSet, byColor map[string]*common.ImageSet) *common.ImageSet {
	if pkg == nil || index < 0 {
		return nil
	}
	if index < len(pkg.RequestDraft.SKCList) {
		skc := &pkg.RequestDraft.SKCList[index]
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
		attrs := pkg.SkcList[index].Attributes
		for _, value := range []string{
			attrs["Color"],
			attrs["color"],
			pkg.SkcList[index].SaleName,
			pkg.SkcList[index].SkcName,
		} {
			if images := byColor[normalizeSDSColorKey(value)]; images != nil {
				return images
			}
		}
		for _, sku := range pkg.SkcList[index].SKUs {
			if images := bySKU[normalizeSDSColorKey(sku.SKU)]; images != nil {
				return images
			}
		}
	}
	return nil
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
