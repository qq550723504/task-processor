package shein

import (
	"regexp"
	"strings"
)

var (
	saleAttributeLeadingScalePattern = regexp.MustCompile(`(?i)\b(eur|eu|us|uk)\s*([0-9])`)
	saleAttributeNoisePattern        = regexp.MustCompile(`(?i)\b(eur|eu|us|uk|size)\b`)
)

func normalizeSaleAttributeValue(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	value = strings.NewReplacer(
		"，", ",",
		"（", "(",
		"）", ")",
		"_", " ",
		"-", " ",
		"/", " ",
	).Replace(value)
	value = saleAttributeLeadingScalePattern.ReplaceAllString(value, `$2`)
	value = saleAttributeNoisePattern.ReplaceAllString(value, " ")
	value = strings.NewReplacer(
		"尺码", " ",
		"尺寸", " ",
		"颜色", " ",
		"颜色分类", " ",
		"码", " ",
	).Replace(value)
	value = trimSaleAttributeCodePrefix(value)
	value = strings.Join(strings.Fields(value), " ")
	return value
}

func comparableAttributeValueForms(value string) []string {
	forms := []string{
		normalizeSaleAttributeValue(value),
		normalizeText(value),
	}

	result := make([]string, 0, len(forms)+4)
	seen := make(map[string]struct{}, len(forms)+4)
	for _, form := range forms {
		if form == "" {
			continue
		}
		result = addComparableAttributeValueForm(result, seen, form)
		for _, alias := range comparableMaterialAliases(form) {
			result = addComparableAttributeValueForm(result, seen, alias)
		}
	}
	return result
}

func addComparableAttributeValueForm(result []string, seen map[string]struct{}, form string) []string {
	form = strings.TrimSpace(form)
	if form == "" {
		return result
	}
	if _, ok := seen[form]; ok {
		return result
	}
	seen[form] = struct{}{}
	return append(result, form)
}

func comparableMaterialAliases(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	switch {
	case containsComparableMaterialTerm(value, "mesh fabric", "mesh", "网布", "网眼布", "飞织布", "飞织", "flyknit", "fly knit"):
		return []string{"mesh fabric", "mesh"}
	default:
		return nil
	}
}

func containsComparableMaterialTerm(value string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}
