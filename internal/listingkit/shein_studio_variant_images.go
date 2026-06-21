package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
)

func normalizeSheinStudioVariantImageSets(input []SheinStudioVariantImageSet) []sheinpub.VariantImageSet {
	result := make([]sheinpub.VariantImageSet, 0, len(input))
	for _, item := range input {
		images := uniqueNonEmptyStrings(item.ImageURLs)
		if len(images) == 0 {
			continue
		}
		result = append(result, sheinpub.VariantImageSet{
			VariantSKU: strings.TrimSpace(item.VariantSKU),
			Color:      strings.TrimSpace(item.Color),
			ImageURLs:  images,
		})
	}
	return result
}

func applyVariantProductImagesToShein(pkg *sheinpub.Package, variantImages []sheinpub.VariantImageSet, sourceImages []string) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || len(variantImages) == 0 {
		return
	}
	byColor := make(map[string]sheinpub.VariantImageSet, len(variantImages))
	bySKU := make(map[string]sheinpub.VariantImageSet, len(variantImages))
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

func findVariantImageSetForSKC(skc sheinpub.SKCRequestDraft, byColor map[string]sheinpub.VariantImageSet, bySKU map[string]sheinpub.VariantImageSet) (sheinpub.VariantImageSet, bool) {
	return sheinpub.FindVariantImageSetForRequestSKC(skc, byColor, bySKU)
}

func findVariantImageSetForSKCPackage(skc sheinpub.SKCPackage, byColor map[string]sheinpub.VariantImageSet, bySKU map[string]sheinpub.VariantImageSet) (sheinpub.VariantImageSet, bool) {
	return sheinpub.FindVariantImageSetForPackageSKC(skc, byColor, bySKU)
}

func normalizeVariantImageKey(value string) string {
	return sheinpub.NormalizeVariantImageKey(value)
}
