package publishing

import "strings"

// SaleDimensionMatches reports whether two source/template sale dimensions are equivalent.
func SaleDimensionMatches(left, right string) bool {
	left = NormalizeSaleDimension(left)
	right = NormalizeSaleDimension(right)
	if left == "" || right == "" {
		return false
	}
	if left == right {
		return true
	}
	return (left == "color" && right == "colour") ||
		(left == "colour" && right == "color")
}

// NormalizeSaleDimension normalizes common source dimension labels.
func NormalizeSaleDimension(value string) string {
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

// ResolvedSaleAttributeValueReady reports whether a resolved sale attribute value has usable IDs.
func ResolvedSaleAttributeValueReady(attributeID int, attributeValueID *int) bool {
	return attributeID > 0 && attributeValueID != nil && *attributeValueID > 0
}
