package studio

import "strings"

const maxDesignReferenceImages = 5

// NormalizeDesignReferenceImageURLs trims, deduplicates, and caps the
// reference image list used by Studio design prompts.
func NormalizeDesignReferenceImageURLs(values []string) []string {
	cleaned := make([]string, 0, min(len(values), maxDesignReferenceImages))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		cleaned = append(cleaned, trimmed)
		if len(cleaned) >= maxDesignReferenceImages {
			break
		}
	}
	return cleaned
}
