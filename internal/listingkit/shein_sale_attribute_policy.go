package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
)

func sheinSecondarySaleAttributeRequired(pkg *SheinPackage) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SaleAttributeResolution == nil {
		return false
	}
	if !sheinHasMultiSKUWithinSingleSKC(pkg) {
		return false
	}
	if !sheinHasSecondarySourceVariation(pkg) {
		return false
	}
	return sheinHasSecondaryTemplateCandidate(pkg.SaleAttributeResolution)
}

func sheinHasMultiSKUWithinSingleSKC(pkg *SheinPackage) bool {
	if pkg == nil {
		return false
	}
	if pkg.DraftPayload != nil {
		for _, skc := range pkg.DraftPayload.SKCList {
			if len(skc.SKUList) > 1 {
				return true
			}
		}
	}
	if pkg.PreviewPayload != nil {
		for _, skc := range pkg.PreviewPayload.SKCList {
			if len(skc.SKUS) > 1 {
				return true
			}
		}
	}
	for _, skc := range pkg.SkcList {
		if len(skc.SKUs) > 1 {
			return true
		}
	}
	return false
}

func sheinHasSecondarySourceVariation(pkg *SheinPackage) bool {
	if pkg == nil || pkg.SaleAttributeResolution == nil {
		return false
	}
	resolution := pkg.SaleAttributeResolution
	if resolution.SecondaryAttributeID > 0 || len(resolution.SKUAttributes) > 0 || len(resolution.SKUValueAssignments) > 0 {
		return true
	}
	sourceName := strings.TrimSpace(resolution.SecondarySourceDimension)
	if sourceName == "" {
		return false
	}
	for _, dimension := range resolution.SourceDimensions {
		if sheinSaleDimensionMatches(sourceName, dimension.Name) && dimension.DistinctCount > 1 {
			return true
		}
	}
	for _, skc := range pkg.SkcList {
		values := map[string]struct{}{}
		for _, sku := range skc.SKUs {
			value := ""
			for key, attrValue := range sku.Attributes {
				if sheinSaleDimensionMatches(sourceName, key) {
					value = strings.TrimSpace(attrValue)
					break
				}
			}
			if value == "" {
				continue
			}
			values[value] = struct{}{}
			if len(values) > 1 {
				return true
			}
		}
	}
	return false
}

func sheinHasSecondaryTemplateCandidate(resolution *SheinSaleAttributeResolution) bool {
	if resolution == nil {
		return false
	}
	sourceName := strings.TrimSpace(resolution.SecondarySourceDimension)
	if sourceName == "" {
		return false
	}
	for _, candidate := range resolution.Candidates {
		if candidate.AttributeID <= 0 || candidate.AttributeID == resolution.PrimaryAttributeID || candidate.SKCScope {
			continue
		}
		if sheinSaleDimensionMatches(sourceName, candidate.SourceDimension) || sheinSaleDimensionMatches(sourceName, candidate.Name) {
			return true
		}
	}
	for _, option := range resolution.TemplateOptions {
		if option.AttributeID <= 0 || option.AttributeID == resolution.PrimaryAttributeID || option.SKCScope {
			continue
		}
		if sheinSaleDimensionMatches(sourceName, option.Name) || sheinSaleDimensionMatches(sourceName, option.NameEn) {
			return true
		}
	}
	return false
}

func sheinSaleDimensionMatches(left, right string) bool {
	left = sheinNormalizeSaleDimension(left)
	right = sheinNormalizeSaleDimension(right)
	if left == "" || right == "" {
		return false
	}
	if left == right {
		return true
	}
	return (left == "color" && right == "colour") ||
		(left == "colour" && right == "color")
}

func sheinNormalizeSaleDimension(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "color", "colour", "颜色", "颜色分类":
		return "color"
	case "size", "尺码", "尺寸", "规格":
		return "size"
	case "quantity", "count", "件数", "数量":
		return "quantity"
	case "style", "style type", "款式", "类型":
		return "style"
	default:
		return value
	}
}
