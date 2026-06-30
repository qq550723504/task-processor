package shein

import sheinmarketpub "task-processor/internal/marketplace/shein/publishing"

// VariantImageSet describes generated product images for a SHEIN variant.
type VariantImageSet = sheinmarketpub.VariantImageSet

// FindVariantImageSetForRequestSKC matches generated variant images to a draft SKC.
func FindVariantImageSetForRequestSKC(skc SKCRequestDraft, byColor map[string]VariantImageSet, bySKU map[string]VariantImageSet) (VariantImageSet, bool) {
	return sheinmarketpub.FindVariantImageSet(variantImageInputFromRequestSKC(skc), byColor, bySKU)
}

// FindVariantImageSetForPackageSKC matches generated variant images to a package SKC.
func FindVariantImageSetForPackageSKC(skc SKCPackage, byColor map[string]VariantImageSet, bySKU map[string]VariantImageSet) (VariantImageSet, bool) {
	return sheinmarketpub.FindVariantImageSet(variantImageInputFromPackageSKC(skc), byColor, bySKU)
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
	return sheinmarketpub.NormalizeVariantImageKey(value)
}

func variantImageInputFromRequestSKC(skc SKCRequestDraft) sheinmarketpub.VariantImageSKCInput {
	input := sheinmarketpub.VariantImageSKCInput{
		SKUCandidates: []string{
			SourceSDSSKUFromSupplierSKU(skc.SupplierCode),
			skc.SupplierCode,
		},
		ColorCandidates: []string{
			skc.SaleName,
			skc.SkcName,
			ResolvedSaleAttributeValue(skc.SaleAttribute),
		},
	}
	for _, sku := range skc.SKUList {
		input.SKUCandidates = append(input.SKUCandidates,
			sku.Attributes["source_sds_sku"],
			SourceSDSSKUFromSupplierSKU(sku.SupplierSKU),
			sku.SupplierSKU,
		)
		input.ColorCandidates = append(input.ColorCandidates, sku.Attributes["Color"])
	}
	return input
}

func variantImageInputFromPackageSKC(skc SKCPackage) sheinmarketpub.VariantImageSKCInput {
	input := sheinmarketpub.VariantImageSKCInput{
		SKUCandidates: []string{
			SourceSDSSKUFromSupplierSKU(skc.SupplierCode),
			skc.SupplierCode,
		},
		ColorCandidates: []string{skc.SaleName, skc.SkcName, skc.Attributes["Color"]},
	}
	for _, sku := range skc.SKUs {
		input.SKUCandidates = append(input.SKUCandidates,
			sku.Attributes["source_sds_sku"],
			SourceSDSSKUFromSupplierSKU(sku.SKU),
			sku.SKU,
		)
	}
	return input
}
