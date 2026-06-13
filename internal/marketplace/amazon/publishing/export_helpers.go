package publishing

import (
	"regexp"
	"strings"
)

func MarketplaceIDByCountry(country string) string {
	switch strings.ToUpper(strings.TrimSpace(country)) {
	case "US", "":
		return "ATVPDKIKX0DER"
	case "CA":
		return "A2EUQ1WTGCTBG2"
	case "MX":
		return "A1AM78C64UM0Y8"
	case "UK", "GB":
		return "A1F83G8C2ARO7P"
	case "DE":
		return "A1PA6795UKMFR9"
	case "FR":
		return "A13V1IB3VIYZZH"
	case "IT":
		return "APJ6JRA9NG5V4"
	case "ES":
		return "A1RKKUPIHCS9HS"
	case "JP":
		return "A1VC38T7YXB528"
	default:
		return "ATVPDKIKX0DER"
	}
}

func NormalizeLanguageTag(language string) string {
	language = strings.TrimSpace(language)
	if language == "" {
		return "en_US"
	}
	return strings.ReplaceAll(language, "-", "_")
}

func SanitizeProductType(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "&", "AND")
	value = regexp.MustCompile(`[^A-Z0-9_]+`).ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return "PRODUCT"
	}
	return value
}

func SanitizeAttributeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "&", " and ")
	value = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	return value
}

func NormalizeDimensionUnit(unit string) string {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "cm", "centimeter", "centimeters":
		return "centimeters"
	case "mm", "millimeter", "millimeters":
		return "millimeters"
	case "in", "inch", "inches":
		return "inches"
	default:
		return strings.ToLower(strings.TrimSpace(unit))
	}
}

func NormalizeWeightUnit(unit string) string {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "kg", "kilogram", "kilograms":
		return "kilograms"
	case "g", "gram", "grams":
		return "grams"
	case "lb", "lbs", "pound", "pounds":
		return "pounds"
	case "oz", "ounce", "ounces":
		return "ounces"
	default:
		return strings.ToLower(strings.TrimSpace(unit))
	}
}

func SanitizeSKU(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = regexp.MustCompile(`[^A-Z0-9_-]+`).ReplaceAllString(value, "-")
	value = strings.Trim(value, "-_")
	if value == "" {
		return "AL-GENERATED"
	}
	return value
}

func CompactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func CloneAttributes(src map[string]any) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
