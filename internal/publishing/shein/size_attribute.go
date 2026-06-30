package shein

import (
	"strings"

	sheinpublishing "task-processor/internal/marketplace/shein/publishing"
)

const defaultSizeSaleAttributeID = 87

func applyProductSizeAttributes(pkg *Package, productSize string) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return false
	}
	attrs := sheinpublishing.BuildSizeAttributesFromProductSize(productSize, collectSizeSaleAttributeRefs(pkg))
	pkg.DraftPayload.SizeAttributeList = attrs
	return len(attrs) > 0
}

func collectSizeSaleAttributeRefs(pkg *Package) []sheinpublishing.SizeSaleAttributeRef {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return nil
	}
	sizeAttributeID := defaultSizeSaleAttributeID
	sizeSourceDimension := "size"
	if pkg.SaleAttributeResolution != nil {
		if pkg.SaleAttributeResolution.SecondaryAttributeID > 0 && sheinpublishing.IsSizeSourceDimension(pkg.SaleAttributeResolution.SecondarySourceDimension) {
			sizeAttributeID = pkg.SaleAttributeResolution.SecondaryAttributeID
		}
		if strings.TrimSpace(pkg.SaleAttributeResolution.SecondarySourceDimension) != "" {
			sizeSourceDimension = pkg.SaleAttributeResolution.SecondarySourceDimension
		}
	}
	var result []sheinpublishing.SizeSaleAttributeRef
	for _, skc := range pkg.DraftPayload.SKCList {
		for _, sku := range skc.SKUList {
			for _, attr := range sku.SaleAttributes {
				if attr.AttributeID != sizeAttributeID || attr.AttributeValueID == nil || *attr.AttributeValueID <= 0 {
					continue
				}
				sizeValue := firstNonEmpty(attr.Value, lookupAttributeValue(sku.Attributes, sizeSourceDimension), lookupAttributeValue(sku.Attributes, "Size"), lookupAttributeValue(sku.Attributes, "尺码"))
				if strings.TrimSpace(sizeValue) == "" {
					continue
				}
				result = append(result, sheinpublishing.SizeSaleAttributeRef{
					SizeValue:        sizeValue,
					AttributeID:      attr.AttributeID,
					AttributeValueID: *attr.AttributeValueID,
				})
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
