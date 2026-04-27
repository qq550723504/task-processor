package listingkit

import (
	"strings"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

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
		if _, exists := byColor[key]; exists {
		} else {
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
			skc := &pkg.RequestDraft.SKCList[skcIndex]
			images := byColor[normalizeSDSColorKey(skc.SkcName)]
			if images == nil {
				images = defaultImages
			}
			skc.ImageInfo = sheinpub.BuildImageDraft(images)
			for skuIndex := range skc.SKUList {
				skuImages := resolveSDSImagesForSKU(&skc.SKUList[skuIndex], bySKU, byColor)
				if skuImages == nil {
					skuImages = images
				}
				skc.SKUList[skuIndex].MainImage = skuImages.MainImage
			}
		}
	}
	for skcIndex := range pkg.SkcList {
		images := byColor[normalizeSDSColorKey(pkg.SkcList[skcIndex].SkcName)]
		if images == nil {
			continue
		}
		pkg.SkcList[skcIndex].MainImageURL = images.MainImage
	}
	pkg.PreviewProduct = sheinpub.BuildPreviewProduct(pkg)
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

func normalizeSDSColorKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "__default__"
	}
	return value
}
