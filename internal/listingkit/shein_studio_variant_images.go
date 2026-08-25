package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
)

func normalizeSheinStudioVariantImageSets(input []SheinStudioVariantImageSet) []sheinpub.VariantImageSet {
	sets := make([]sheinpub.VariantImageSet, 0, len(input))
	for _, item := range input {
		sets = append(sets, sheinpub.VariantImageSet{
			VariantSKU: strings.TrimSpace(item.VariantSKU),
			Color:      strings.TrimSpace(item.Color),
			ImageURLs:  item.ImageURLs,
		})
	}
	return sheinpub.NormalizeVariantImageSets(sets)
}

func applyVariantProductImagesToShein(pkg *sheinpub.Package, variantImages []sheinpub.VariantImageSet, sourceImages []string) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || len(variantImages) == 0 {
		return
	}
	byColor := make(map[string]sheinpub.VariantImageSet, len(variantImages))
	bySKU := make(map[string]sheinpub.VariantImageSet, len(variantImages))
	for _, item := range variantImages {
		if key := sheinpub.NormalizeVariantImageKey(item.Color); key != "" {
			byColor[key] = item
		}
		if key := sheinpub.NormalizeVariantImageKey(item.VariantSKU); key != "" {
			bySKU[key] = item
		}
	}
	if pkg.DraftPayload != nil {
		for skcIndex := range pkg.DraftPayload.SKCList {
			skc := &pkg.DraftPayload.SKCList[skcIndex]
			if item, ok := sheinpub.FindVariantImageSetForRequestSKC(*skc, byColor, bySKU); ok {
				images := sheinpub.ImageSetFromAIProductImages(item.ImageURLs, sourceImages)
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
		if item, ok := sheinpub.FindVariantImageSetForPackageSKC(*skc, byColor, bySKU); ok && len(item.ImageURLs) > 0 {
			skc.MainImageURL = item.ImageURLs[0]
		}
	}
	preview := sheinpub.BuildPreviewProduct(pkg)
	sheinpub.SetPreviewPayload(pkg, preview)
}
