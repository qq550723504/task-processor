package common

import (
	"fmt"
	"regexp"
	"strings"

	"task-processor/internal/productenrich"
)

const defaultVariantSKU = "DEFAULT-001"

func buildFallbackVariant(canonical *productenrich.CanonicalProduct) []Variant {
	if canonical == nil {
		return nil
	}

	attributes := flattenCanonicalAttributes(canonical.Attributes)
	colorKey, colorValues := resolveVariantAttributeOptions(attributes, []string{"color", "colour", "颜色"})
	sizeKey, sizeValues := resolveVariantAttributeOptions(attributes, []string{"size", "尺寸", "尺码"})

	var price *Price
	if guessed := inferVariantPriceFromAttributes(canonical.Attributes); guessed != nil {
		price = guessed
	}

	var imageURL string
	if len(canonical.Images) > 0 {
		imageURL = strings.TrimSpace(canonical.Images[0].URL)
	}

	if len(colorValues) == 0 {
		colorValues = []string{""}
	}
	if len(sizeValues) == 0 {
		sizeValues = []string{""}
	}

	totalVariants := len(colorValues) * len(sizeValues)
	variants := make([]Variant, 0, totalVariants)
	index := 0
	for _, color := range colorValues {
		for _, size := range sizeValues {
			variantAttributes := CloneMap(attributes)
			if colorKey != "" && strings.TrimSpace(color) != "" {
				variantAttributes[colorKey] = color
			}
			if sizeKey != "" && strings.TrimSpace(size) != "" {
				variantAttributes[sizeKey] = size
			}

			variants = append(variants, Variant{
				SKU:        buildFallbackVariantSKU(index, totalVariants, color, size),
				Attributes: variantAttributes,
				Price:      clonePrice(price),
				Stock:      0,
				Image:      imageURL,
				IsDefault:  index == 0,
			})
			index++
		}
	}
	if len(variants) == 0 {
		return []Variant{{
			SKU:        defaultVariantSKU,
			Attributes: attributes,
			Price:      clonePrice(price),
			Stock:      0,
			Image:      imageURL,
			IsDefault:  true,
		}}
	}
	return variants
}

func inferVariantPriceFromAttributes(attributes map[string]productenrich.CanonicalAttribute) *Price {
	if len(attributes) == 0 {
		return nil
	}

	currency := firstAttributeValue(attributes, "currency", "price_currency")
	amount := ParseFloatDefault(firstAttributeValue(attributes, "price", "sale_price", "amount"))
	compareAt := ParseFloatDefault(firstAttributeValue(attributes, "compare_at", "market_price", "original_price"))
	costPrice := ParseFloatDefault(firstAttributeValue(attributes, "cost_price", "purchase_price"))

	if amount <= 0 && costPrice <= 0 && compareAt <= 0 {
		return nil
	}

	return &Price{
		Currency:  FirstNonEmpty(currency, "CNY"),
		Amount:    amount,
		CostPrice: costPrice,
	}
}

func flattenCanonicalAttributes(attributes map[string]productenrich.CanonicalAttribute) map[string]string {
	if len(attributes) == 0 {
		return nil
	}
	result := make(map[string]string, len(attributes))
	for key, value := range attributes {
		if strings.TrimSpace(value.Value) == "" {
			continue
		}
		result[key] = value.Value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func resolveVariantAttributeOptions(attributes map[string]string, keys []string) (string, []string) {
	for _, key := range keys {
		value := strings.TrimSpace(attributes[key])
		if value == "" {
			continue
		}
		options := splitVariantOptions(value)
		if len(options) == 0 {
			continue
		}
		return key, options
	}
	return "", nil
}

var variantOptionSplitter = regexp.MustCompile(`\s*(?:,|，|/|\||;|；|\n|\r\n|、)\s*`)

func splitVariantOptions(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := variantOptionSplitter.Split(value, -1)
	result := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		normalized := normalizeVariantOption(part)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, part)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeVariantOption(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func buildFallbackVariantSKU(index int, totalVariants int, color, size string) string {
	if totalVariants <= 1 {
		return defaultVariantSKU
	}
	parts := []string{defaultVariantSKU}
	if token := normalizeVariantToken(color); token != "" {
		parts = append(parts, token)
	}
	if token := normalizeVariantToken(size); token != "" {
		parts = append(parts, token)
	}
	if len(parts) == 1 && index > 0 {
		parts = append(parts, fmt.Sprintf("%02d", index+1))
	}
	return strings.Join(parts, "-")
}

func normalizeVariantToken(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	token := strings.Trim(b.String(), "-")
	if token == "" {
		return ""
	}
	return token
}

func clonePrice(price *Price) *Price {
	if price == nil {
		return nil
	}
	cloned := *price
	return &cloned
}

func firstAttributeValue(attributes map[string]productenrich.CanonicalAttribute, keys ...string) string {
	for _, key := range keys {
		if value, ok := attributes[key]; ok && strings.TrimSpace(value.Value) != "" {
			return strings.TrimSpace(value.Value)
		}
	}
	return ""
}
