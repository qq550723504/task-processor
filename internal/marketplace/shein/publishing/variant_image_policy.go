package publishing

import "strings"

// VariantImageSet describes generated product images for a SHEIN variant.
type VariantImageSet struct {
	VariantSKU string
	Color      string
	ImageURLs  []string
}

// VariantImageSKCInput is the neutral SKC shape needed for variant image matching.
type VariantImageSKCInput struct {
	SKUCandidates   []string
	ColorCandidates []string
}

// FindVariantImageSet matches generated variant images to an SKC by SKU first, then color.
func FindVariantImageSet(input VariantImageSKCInput, byColor map[string]VariantImageSet, bySKU map[string]VariantImageSet) (VariantImageSet, bool) {
	for _, candidate := range input.SKUCandidates {
		if item, ok := bySKU[NormalizeVariantImageKey(candidate)]; ok {
			return item, true
		}
	}
	for _, candidate := range input.ColorCandidates {
		if item, ok := byColor[NormalizeVariantImageKey(candidate)]; ok {
			return item, true
		}
	}
	return VariantImageSet{}, false
}

// NormalizeVariantImageKey returns the lookup key for variant image SKU and color matching.
func NormalizeVariantImageKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
