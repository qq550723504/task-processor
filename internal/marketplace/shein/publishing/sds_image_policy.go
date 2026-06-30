package publishing

import (
	"strings"

	common "task-processor/internal/publishing/common"
)

const DefaultSDSImageKey = "__default__"

// SDSImageLookupInput is the neutral input for resolving SDS images by SKU then color.
type SDSImageLookupInput struct {
	SKUCandidates   []string
	ColorCandidates []string
}

// RegisterSDSVariantImageSet indexes a variant image set by SKU and color.
func RegisterSDSVariantImageSet(bySKU map[string]*common.ImageSet, byColor map[string]*common.ImageSet, sku string, color string, images *common.ImageSet, overwrite bool) {
	if images == nil {
		return
	}
	if key := NormalizeSDSImageKey(color); key != DefaultSDSImageKey {
		if overwrite || byColor[key] == nil {
			byColor[key] = images
		}
	}
	if key := NormalizeSDSImageKey(sku); key != DefaultSDSImageKey {
		if overwrite || bySKU[key] == nil {
			bySKU[key] = images
		}
	}
}

// FirstSDSImageSet returns the first non-nil image set from a map.
func FirstSDSImageSet(values map[string]*common.ImageSet) *common.ImageSet {
	for _, images := range values {
		if images != nil {
			return images
		}
	}
	return nil
}

// ResolveSDSImages resolves SDS images by source SKU, supplier SKU, and color.
func ResolveSDSImages(input SDSImageLookupInput, bySKU map[string]*common.ImageSet, byColor map[string]*common.ImageSet) *common.ImageSet {
	for _, value := range input.SKUCandidates {
		if images := LookupSDSImagesBySKU(bySKU, value); images != nil {
			return images
		}
	}
	for _, value := range input.ColorCandidates {
		if images := byColor[NormalizeSDSImageKey(value)]; images != nil {
			return images
		}
	}
	return nil
}

// SourceSDSSKUFromSupplierSKU strips generated supplier SKU suffixes to recover the source SDS SKU.
func SourceSDSSKUFromSupplierSKU(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if index := strings.LastIndex(value, "-"); index > 0 {
		return strings.TrimSpace(value[:index])
	}
	return value
}

// ImageSetFromSDSMockups builds a publishing image set from SDS rendered mockups.
func ImageSetFromSDSMockups(mockups []string, sourceImages []string) *common.ImageSet {
	mockups = UniqueNonEmptySDSImageStrings(mockups)
	if len(mockups) == 0 {
		return nil
	}
	images := &common.ImageSet{
		MainImage:    mockups[0],
		SourceImages: UniqueNonEmptySDSImageStrings(sourceImages),
	}
	if len(mockups) > 1 {
		images.Gallery = append([]string(nil), mockups[1:]...)
	}
	return images
}

// MergeSDSImageSet merges an additional SDS image set into an existing one.
func MergeSDSImageSet(existing *common.ImageSet, next *common.ImageSet) *common.ImageSet {
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
	existing.Gallery = AppendUniqueSDSImageURLs(existing.Gallery, next.MainImage)
	existing.Gallery = AppendUniqueSDSImageURLs(existing.Gallery, next.Gallery...)
	return existing
}

// NormalizeSDSImageKey returns a stable key for SDS SKU/color image lookups.
func NormalizeSDSImageKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return DefaultSDSImageKey
	}
	return value
}

// LookupSDSImagesBySKU resolves an image set from a SKU map with SDS key normalization.
func LookupSDSImagesBySKU(bySKU map[string]*common.ImageSet, value string) *common.ImageSet {
	if images := bySKU[NormalizeSDSImageKey(value)]; images != nil {
		return images
	}
	return nil
}

// UniqueNonEmptySDSImageStrings returns unique non-empty image strings preserving order.
func UniqueNonEmptySDSImageStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
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

// AppendUniqueSDSImageURLs appends unique non-empty image URLs preserving order.
func AppendUniqueSDSImageURLs(existing []string, additions ...string) []string {
	seen := make(map[string]struct{}, len(existing)+len(additions))
	result := make([]string, 0, len(existing)+len(additions))
	for _, item := range existing {
		key := strings.TrimSpace(item)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	for _, item := range additions {
		key := strings.TrimSpace(item)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}
