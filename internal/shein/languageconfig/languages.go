// Package languageconfig resolves the ordered product languages for a SHEIN task.
package languageconfig

import (
	"strings"

	"task-processor/internal/shein/api/product"
	"task-processor/internal/shein/submitprep"
)

// Normalize filters and deduplicates enabled language rows while preserving order.
func Normalize(items []product.LanguageListItem) []string {
	languages := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		language := strings.ToLower(strings.TrimSpace(item.LanguageAbbr))
		if item.InputMode <= 0 || language == "" {
			continue
		}
		if _, ok := seen[language]; ok {
			continue
		}
		seen[language] = struct{}{}
		languages = append(languages, language)
	}
	return languages
}

// Resolve returns normalized API languages or the compatible region fallback.
func Resolve(items []product.LanguageListItem, region string) []string {
	if languages := Normalize(items); len(languages) > 0 {
		return append([]string(nil), languages...)
	}
	languages := submitprep.GetTargetLanguagesByRegion(strings.ToUpper(strings.TrimSpace(region)))
	if len(languages) == 0 {
		languages = []string{"en"}
	}
	return append([]string(nil), languages...)
}
