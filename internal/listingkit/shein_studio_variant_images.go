package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
)

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
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
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
	if pkg.DraftPayload != nil {
		for skcIndex := range pkg.DraftPayload.SKCList {
			skc := &pkg.DraftPayload.SKCList[skcIndex]
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
	preview := sheinpub.BuildPreviewProduct(pkg)
	sheinpub.SetPreviewPayload(pkg, preview)
}

func findVariantImageSetForSKC(skc sheinpub.SKCRequestDraft, byColor map[string]SheinStudioVariantImageSet, bySKU map[string]SheinStudioVariantImageSet) (SheinStudioVariantImageSet, bool) {
	for _, candidate := range variantImageSKUCandidatesFromRequestSKC(skc) {
		if item, ok := bySKU[normalizeVariantImageKey(candidate)]; ok {
			return item, true
		}
	}
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
		if item, ok := byColor[normalizeVariantImageKey(sku.Attributes["Color"])]; ok {
			return item, true
		}
	}
	return SheinStudioVariantImageSet{}, false
}

func findVariantImageSetForSKCPackage(skc sheinpub.SKCPackage, byColor map[string]SheinStudioVariantImageSet, bySKU map[string]SheinStudioVariantImageSet) (SheinStudioVariantImageSet, bool) {
	for _, candidate := range variantImageSKUCandidatesFromPackageSKC(skc) {
		if item, ok := bySKU[normalizeVariantImageKey(candidate)]; ok {
			return item, true
		}
	}
	for _, candidate := range []string{skc.SaleName, skc.SkcName, skc.Attributes["Color"]} {
		if item, ok := byColor[normalizeVariantImageKey(candidate)]; ok {
			return item, true
		}
	}
	return SheinStudioVariantImageSet{}, false
}

func variantImageSKUCandidatesFromRequestSKC(skc sheinpub.SKCRequestDraft) []string {
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

func variantImageSKUCandidatesFromPackageSKC(skc sheinpub.SKCPackage) []string {
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

func saleAttributeValue(attr *sheinpub.ResolvedSaleAttribute) string {
	if attr == nil {
		return ""
	}
	return attr.Value
}

func normalizeVariantImageKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
