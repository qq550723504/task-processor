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

// NormalizeVariantImageSets trims variant identity fields, deduplicates image
// URLs, and drops sets that contain no usable images.
func NormalizeVariantImageSets(input []VariantImageSet) []VariantImageSet {
	result := make([]VariantImageSet, 0, len(input))
	for _, item := range input {
		images := uniqueNonEmptyImageStrings(item.ImageURLs)
		if len(images) == 0 {
			continue
		}
		result = append(result, VariantImageSet{
			VariantSKU: strings.TrimSpace(item.VariantSKU),
			Color:      strings.TrimSpace(item.Color),
			ImageURLs:  images,
		})
	}
	return result
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

func uniqueNonEmptyImageStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
