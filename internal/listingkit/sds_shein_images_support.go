package listingkit

import (
	"strings"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

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
	sheinpub.RegisterSDSVariantImageSet(bySKU, byColor, sku, color, images, overwrite)
}

func firstSDSImageSet(values map[string]*common.ImageSet) *common.ImageSet {
	return sheinpub.FirstSDSImageSet(values)
}

func resolveSDSImagesForSKC(pkg *sheinpub.Package, index int, bySKU map[string]*common.ImageSet, byColor map[string]*common.ImageSet) *common.ImageSet {
	return sheinpub.ResolveSDSImagesForSKC(pkg, index, bySKU, byColor)
}

func resolveSDSImagesForSKU(sku *sheinpub.SKUDraft, bySKU map[string]*common.ImageSet, byColor map[string]*common.ImageSet) *common.ImageSet {
	return sheinpub.ResolveSDSImagesForSKU(sku, bySKU, byColor)
}

func sourceSDSSKUFromSupplierSKU(value string) string {
	return sheinpub.SourceSDSSKUFromSupplierSKU(value)
}

func imageSetFromSDSMockups(mockups []string, sourceImages []string) *common.ImageSet {
	return sheinpub.ImageSetFromSDSMockups(mockups, sourceImages)
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
	return sheinpub.MergeSDSImageSet(existing, next)
}

func normalizeSDSColorKey(value string) string {
	return sheinpub.NormalizeSDSImageKey(value)
}
