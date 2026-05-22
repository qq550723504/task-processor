package shein

import (
	"slices"
	"strings"
)

func countTemplateValueFits(index *templateIndex, templateName string, values []string) (int, int) {
	if index == nil || strings.TrimSpace(templateName) == "" {
		return 0, 0
	}
	attr := index.FindAttribute(templateName)
	if attr == nil {
		return 0, 0
	}
	return countTemplateValueFitsForAttribute(*attr, values)
}

func uniqueNormalizedValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	seen := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		normalized := normalizeText(trimmed)
		if slices.Contains(seen, normalized) {
			continue
		}
		seen = append(seen, normalized)
		result = append(result, trimmed)
	}
	return result
}
