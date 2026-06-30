package shein

import (
	"strings"

	sheinmarketpub "task-processor/internal/marketplace/shein/publishing"
	common "task-processor/internal/publishing/common"
)

const defaultSDSImageKey = sheinmarketpub.DefaultSDSImageKey

// RegisterSDSVariantImageSet indexes a variant image set by SKU and color.
func RegisterSDSVariantImageSet(bySKU map[string]*common.ImageSet, byColor map[string]*common.ImageSet, sku string, color string, images *common.ImageSet, overwrite bool) {
	sheinmarketpub.RegisterSDSVariantImageSet(bySKU, byColor, sku, color, images, overwrite)
}

// FirstSDSImageSet returns the first non-nil image set from a map.
func FirstSDSImageSet(values map[string]*common.ImageSet) *common.ImageSet {
	return sheinmarketpub.FirstSDSImageSet(values)
}

// ResolveSDSImagesForSKC resolves SDS images for an SKC by source SKU, supplier SKU, and color.
func ResolveSDSImagesForSKC(pkg *Package, index int, bySKU map[string]*common.ImageSet, byColor map[string]*common.ImageSet) *common.ImageSet {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || index < 0 {
		return nil
	}
	if index < len(pkg.DraftPayload.SKCList) {
		skc := &pkg.DraftPayload.SKCList[index]
		if images := sheinmarketpub.ResolveSDSImages(sheinmarketpub.SDSImageLookupInput{
			SKUCandidates: sdsSKUCandidatesFromRequestSKC(skc),
			ColorCandidates: []string{
				skcSaleAttributeValue(skc.SaleAttribute),
				skcColorFromDraft(skc),
			},
		}, bySKU, byColor); images != nil {
			return images
		}
	}
	if index < len(pkg.SkcList) {
		skc := &pkg.SkcList[index]
		attrs := skc.Attributes
		if images := sheinmarketpub.ResolveSDSImages(sheinmarketpub.SDSImageLookupInput{
			SKUCandidates: sdsSKUCandidatesFromPackageSKC(skc),
			ColorCandidates: []string{
				attrs["Color"],
				attrs["color"],
				skc.SaleName,
				skc.SkcName,
			},
		}, bySKU, byColor); images != nil {
			return images
		}
	}
	return nil
}

// ResolveSDSImagesForSKU resolves SDS images for an SKU by source SKU and color attributes.
func ResolveSDSImagesForSKU(sku *SKUDraft, bySKU map[string]*common.ImageSet, byColor map[string]*common.ImageSet) *common.ImageSet {
	if sku == nil {
		return nil
	}
	return sheinmarketpub.ResolveSDSImages(sheinmarketpub.SDSImageLookupInput{
		SKUCandidates: []string{
			SourceSDSSKUFromSupplierSKU(sku.SupplierSKU),
			sku.Attributes["source_sds_sku"],
		},
		ColorCandidates: []string{
			sku.Attributes["Color"],
			sku.Attributes["color"],
		},
	}, bySKU, byColor)
}

// SourceSDSSKUFromSupplierSKU strips generated supplier SKU suffixes to recover the source SDS SKU.
func SourceSDSSKUFromSupplierSKU(value string) string {
	return sheinmarketpub.SourceSDSSKUFromSupplierSKU(value)
}

// ImageSetFromSDSMockups builds a publishing image set from SDS rendered mockups.
func ImageSetFromSDSMockups(mockups []string, sourceImages []string) *common.ImageSet {
	return sheinmarketpub.ImageSetFromSDSMockups(mockups, sourceImages)
}

// MergeSDSImageSet merges an additional SDS image set into an existing one.
func MergeSDSImageSet(existing *common.ImageSet, next *common.ImageSet) *common.ImageSet {
	return sheinmarketpub.MergeSDSImageSet(existing, next)
}

// NormalizeSDSImageKey returns a stable key for SDS SKU/color image lookups.
func NormalizeSDSImageKey(value string) string {
	return sheinmarketpub.NormalizeSDSImageKey(value)
}

func lookupSDSImagesBySKU(bySKU map[string]*common.ImageSet, value string) *common.ImageSet {
	return sheinmarketpub.LookupSDSImagesBySKU(bySKU, value)
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
	return sheinmarketpub.UniqueNonEmptySDSImageStrings(values)
}

func appendUniqueSDSImageURLs(existing []string, additions ...string) []string {
	return sheinmarketpub.AppendUniqueSDSImageURLs(existing, additions...)
}
