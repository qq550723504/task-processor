package enrich

import (
	"fmt"
	"slices"
	"strings"

	productenrich "task-processor/internal/productenrich"
)

func variantsFromScrapedData(scraped *productenrich.ScrapedData) []productenrich.ProductVariant {
	if scraped == nil || len(scraped.Variants) == 0 {
		return nil
	}

	variants := make([]productenrich.ProductVariant, 0, len(scraped.Variants))
	for idx, variant := range scraped.Variants {
		normalized := productenrich.ProductVariant{
			SKU:        strings.TrimSpace(variant.SKU),
			Attributes: normalizeVariantAttributes(variant.Attributes),
			Stock:      variant.Stock,
			Images:     cloneNonEmptyStrings(variant.Images),
			Barcode:    strings.TrimSpace(variant.Barcode),
			IsDefault:  variant.IsDefault,
		}

		if normalized.SKU == "" {
			normalized.SKU = buildScrapedVariantSKU(idx, normalized.Attributes)
		}
		if len(normalized.Images) == 0 && len(scraped.Images) > 0 {
			normalized.Images = []string{scraped.Images[0]}
		}
		normalized.Price = normalizeVariantPrice(variant.Price, scraped.Price)
		variants = append(variants, normalized)
	}

	ensureDefaultVariant(variants)
	return variants
}

func normalizeVariantAttributes(attributes map[string]string) map[string]string {
	if len(attributes) == 0 {
		return map[string]string{}
	}

	normalized := make(map[string]string, len(attributes))
	for key, value := range attributes {
		name := strings.TrimSpace(key)
		val := strings.TrimSpace(value)
		if name == "" || val == "" {
			continue
		}
		normalized[name] = val
	}
	return normalized
}

func normalizeVariantPrice(price *productenrich.PriceInfo, fallback float64) *productenrich.PriceInfo {
	var normalized productenrich.PriceInfo
	if price != nil {
		normalized = *price
	}

	if normalized.Amount <= 0 && fallback > 0 {
		normalized.Amount = fallback
	}
	if normalized.CostPrice <= 0 && normalized.Amount > 0 {
		normalized.CostPrice = normalized.Amount
	}
	if normalized.Currency == "" && (normalized.Amount > 0 || normalized.CostPrice > 0) {
		normalized.Currency = "CNY"
	}
	if normalized.Amount <= 0 && normalized.CostPrice <= 0 && normalized.Currency == "" {
		return nil
	}
	return &normalized
}

func buildScrapedVariantSKU(index int, attributes map[string]string) string {
	if len(attributes) == 0 {
		return fmt.Sprintf("SCRAPED-%03d", index+1)
	}

	keys := make([]string, 0, len(attributes))
	for key := range attributes {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	parts := []string{"SCRAPED"}
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ",", "-", "|", "-", ";", "-")
	for _, key := range keys {
		token := strings.ToUpper(strings.TrimSpace(attributes[key]))
		token = replacer.Replace(token)
		if token == "" {
			continue
		}
		parts = append(parts, token)
	}
	if len(parts) == 1 {
		return fmt.Sprintf("SCRAPED-%03d", index+1)
	}
	return strings.Join(parts, "-")
}

func cloneNonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	cloned := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		cloned = append(cloned, trimmed)
	}
	if len(cloned) == 0 {
		return nil
	}
	return cloned
}
