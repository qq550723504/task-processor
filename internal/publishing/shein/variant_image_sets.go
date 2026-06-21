package shein

import "strings"

// VariantImageSet describes generated product images for a SHEIN variant.
type VariantImageSet struct {
	VariantSKU string
	Color      string
	ImageURLs  []string
}

// FindVariantImageSetForRequestSKC matches generated variant images to a draft SKC.
func FindVariantImageSetForRequestSKC(skc SKCRequestDraft, byColor map[string]VariantImageSet, bySKU map[string]VariantImageSet) (VariantImageSet, bool) {
	for _, candidate := range variantImageSKUCandidatesFromRequestSKC(skc) {
		if item, ok := bySKU[NormalizeVariantImageKey(candidate)]; ok {
			return item, true
		}
	}
	for _, candidate := range []string{
		skc.SaleName,
		skc.SkcName,
		ResolvedSaleAttributeValue(skc.SaleAttribute),
	} {
		if item, ok := byColor[NormalizeVariantImageKey(candidate)]; ok {
			return item, true
		}
	}
	for _, sku := range skc.SKUList {
		if item, ok := byColor[NormalizeVariantImageKey(sku.Attributes["Color"])]; ok {
			return item, true
		}
	}
	return VariantImageSet{}, false
}

// FindVariantImageSetForPackageSKC matches generated variant images to a package SKC.
func FindVariantImageSetForPackageSKC(skc SKCPackage, byColor map[string]VariantImageSet, bySKU map[string]VariantImageSet) (VariantImageSet, bool) {
	for _, candidate := range variantImageSKUCandidatesFromPackageSKC(skc) {
		if item, ok := bySKU[NormalizeVariantImageKey(candidate)]; ok {
			return item, true
		}
	}
	for _, candidate := range []string{skc.SaleName, skc.SkcName, skc.Attributes["Color"]} {
		if item, ok := byColor[NormalizeVariantImageKey(candidate)]; ok {
			return item, true
		}
	}
	return VariantImageSet{}, false
}

// ResolvedSaleAttributeValue returns the display value of a resolved sale attribute.
func ResolvedSaleAttributeValue(attr *ResolvedSaleAttribute) string {
	if attr == nil {
		return ""
	}
	return attr.Value
}

// NormalizeVariantImageKey returns the lookup key for variant image SKU and color matching.
func NormalizeVariantImageKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func variantImageSKUCandidatesFromRequestSKC(skc SKCRequestDraft) []string {
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

func variantImageSKUCandidatesFromPackageSKC(skc SKCPackage) []string {
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
