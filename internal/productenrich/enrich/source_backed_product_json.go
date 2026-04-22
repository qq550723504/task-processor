package enrich

import (
	"sort"
	"strings"

	productenrich "task-processor/internal/productenrich"
)

func applySourceBackedAttributes(result *productenrich.ProductJSON, analysis *productenrich.ProductAnalysis) {
	if result == nil || analysis == nil {
		return
	}

	attributes := buildSourceBackedAttributes(analysis)
	if len(attributes) == 0 {
		return
	}
	result.Attributes = attributes
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
