package shein

import (
	"strings"

	common "task-processor/internal/publishing/common"
)

const defaultSDSImageKey = "__default__"

// RegisterSDSVariantImageSet indexes a variant image set by SKU and color.
func RegisterSDSVariantImageSet(bySKU map[string]*common.ImageSet, byColor map[string]*common.ImageSet, sku string, color string, images *common.ImageSet, overwrite bool) {
	if images == nil {
		return
	}
	if key := NormalizeSDSImageKey(color); key != defaultSDSImageKey {
		if overwrite || byColor[key] == nil {
			byColor[key] = images
		}
	}
	if key := NormalizeSDSImageKey(sku); key != defaultSDSImageKey {
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

// ResolveSDSImagesForSKC resolves SDS images for an SKC by source SKU, supplier SKU, and color.
func ResolveSDSImagesForSKC(pkg *Package, index int, bySKU map[string]*common.ImageSet, byColor map[string]*common.ImageSet) *common.ImageSet {
	pkg = NormalizePackageSemanticFields(pkg)
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
			if images := byColor[NormalizeSDSImageKey(value)]; images != nil {
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
			if images := byColor[NormalizeSDSImageKey(value)]; images != nil {
				return images
			}
		}
	}
	return nil
}

// ResolveSDSImagesForSKU resolves SDS images for an SKU by source SKU and color attributes.
func ResolveSDSImagesForSKU(sku *SKUDraft, bySKU map[string]*common.ImageSet, byColor map[string]*common.ImageSet) *common.ImageSet {
	if sku == nil {
		return nil
	}
	if images := bySKU[NormalizeSDSImageKey(SourceSDSSKUFromSupplierSKU(sku.SupplierSKU))]; images != nil {
		return images
	}
	if images := bySKU[NormalizeSDSImageKey(sku.Attributes["source_sds_sku"])]; images != nil {
		return images
	}
	if images := byColor[NormalizeSDSImageKey(sku.Attributes["Color"])]; images != nil {
		return images
	}
	if images := byColor[NormalizeSDSImageKey(sku.Attributes["color"])]; images != nil {
		return images
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
	mockups = uniqueNonEmptySDSImageStrings(mockups)
	if len(mockups) == 0 {
		return nil
	}
	images := &common.ImageSet{
		MainImage:    mockups[0],
		SourceImages: uniqueNonEmptySDSImageStrings(sourceImages),
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
	existing.Gallery = appendUniqueSDSImageURLs(existing.Gallery, next.MainImage)
	existing.Gallery = appendUniqueSDSImageURLs(existing.Gallery, next.Gallery...)
	return existing
}

// NormalizeSDSImageKey returns a stable key for SDS SKU/color image lookups.
func NormalizeSDSImageKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return defaultSDSImageKey
	}
	return value
}

func lookupSDSImagesBySKU(bySKU map[string]*common.ImageSet, value string) *common.ImageSet {
	if images := bySKU[NormalizeSDSImageKey(value)]; images != nil {
		return images
	}
	return nil
}

func sdsSKUCandidatesFromRequestSKC(skc *SKCRequestDraft) []string {
	if skc == nil {
		return nil
	}
	values := []string{
		SourceSDSSKUFromSupplierSKU(skc.SupplierCode),
		skc.SupplierCode,
	}
	for _, sku := range skc.SKUList {
		values = append(values,
			sku.Attributes["source_sds_sku"],
			SourceSDSSKUFromSupplierSKU(sku.SupplierSKU),
			sku.SupplierSKU,
		)
	}
	return values
}

func sdsSKUCandidatesFromPackageSKC(skc *SKCPackage) []string {
	if skc == nil {
		return nil
	}
	values := []string{
		SourceSDSSKUFromSupplierSKU(skc.SupplierCode),
		skc.SupplierCode,
	}
	for _, sku := range skc.SKUs {
		values = append(values,
			sku.Attributes["source_sds_sku"],
			SourceSDSSKUFromSupplierSKU(sku.SKU),
			sku.SKU,
		)
	}
	return values
}

func skcSaleAttributeValue(attribute *ResolvedSaleAttribute) string {
	if attribute == nil {
		return ""
	}
	return attribute.Value
}

func skcColorFromDraft(skc *SKCRequestDraft) string {
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

func uniqueNonEmptySDSImageStrings(values []string) []string {
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

func appendUniqueSDSImageURLs(existing []string, additions ...string) []string {
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
