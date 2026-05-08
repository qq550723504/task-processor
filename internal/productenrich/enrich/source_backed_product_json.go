package enrich

import (
	"sort"
	"strings"

	"task-processor/internal/catalog/canonical"
	productenrich "task-processor/internal/productenrich"
)

func applySourceBackedAttributes(result *productenrich.ProductJSON, analysis *productenrich.ProductAnalysis) {
	if result == nil || analysis == nil {
		return
	}

	applySourceBackedCoreFields(result, analysis)

	attributes := buildSourceBackedAttributes(analysis)
	if len(attributes) == 0 {
		return
	}
	result.Attributes = attributes
}

func applySourceBackedCoreFields(result *productenrich.ProductJSON, analysis *productenrich.ProductAnalysis) {
	if result == nil || analysis == nil || analysis.ScrapedData == nil {
		return
	}

	scraped := analysis.ScrapedData
	if title := strings.TrimSpace(scraped.Title); title != "" {
		result.Title = title
	}
	if categoryPath := normalizeScrapedCategoryPath(scraped.Category); len(categoryPath) > 0 {
		result.Category = append([]string(nil), categoryPath...)
	}
	if description := strings.TrimSpace(scraped.Description); description != "" {
		result.Description = description
	}
	if len(scraped.Images) > 0 {
		result.Images = append([]string(nil), scraped.Images...)
	}
	if len(scraped.VariantDimensions) > 0 {
		result.VariantDimensions = append([]canonical.ScrapedVariantDimension(nil), scraped.VariantDimensions...)
	}
	if len(scraped.Variants) > 0 {
		result.Variants = append([]productenrich.ProductVariant(nil), scraped.Variants...)
	}
}

func buildSourceBackedAttributes(analysis *productenrich.ProductAnalysis) map[string]string {
	if analysis == nil {
		return nil
	}

	attributes := make(map[string]string)
	if analysis.ScrapedData != nil && len(analysis.ScrapedData.Specs) > 0 {
		mergeStringMap(attributes, analysis.ScrapedData.Specs)
		return attributes
	}

	if analysis.TextAttributes != nil {
		mergeStringMap(attributes, analysis.TextAttributes.Attributes)
	}
	if analysis.Representation != nil {
		mergeStringMap(attributes, analysis.Representation.Attributes)
	}

	if len(attributes) == 0 {
		return nil
	}
	return attributes
}

func mergeStringMap(target map[string]string, source map[string]string) {
	if len(source) == 0 {
		return
	}

	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		normalizedKey := strings.TrimSpace(key)
		normalizedValue := strings.TrimSpace(source[key])
		if normalizedKey == "" || normalizedValue == "" {
			continue
		}
		if _, exists := target[normalizedKey]; exists {
			continue
		}
		target[normalizedKey] = normalizedValue
	}
}
