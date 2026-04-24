package enrich

import (
	"strings"

	common "task-processor/internal/publishing/common"
)

func normalizeScrapedCategoryPath(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	replacer := strings.NewReplacer(
		"->", "|",
		">", "|",
		"＞", "|",
		"/", "|",
		"\\", "|",
		"›", "|",
		"»", "|",
	)
	parts := strings.Split(replacer.Replace(raw), "|")
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		normalized = append(normalized, part)
	}
	return common.UniqueStrings(normalized)
}
