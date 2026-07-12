// Package namelimit normalizes and applies SHEIN's per-language name limits.
package namelimit

import (
	"strings"

	"task-processor/internal/shein/api/product"
)

// Limits maps normalized language codes to maximum Unicode character counts.
type Limits map[string]int

// Normalize converts API items into validated, case-insensitive limits.
func Normalize(items []product.NameLengthConfigItem) Limits {
	limits := make(Limits)
	for _, item := range items {
		language := strings.ToLower(strings.TrimSpace(item.Language))
		if language == "" || item.MaxLength <= 0 {
			continue
		}
		limits[language] = item.MaxLength
	}
	return limits
}

// Max returns the configured maximum for a language.
func (l Limits) Max(language string) (int, bool) {
	maxLength, ok := l[strings.ToLower(strings.TrimSpace(language))]
	return maxLength, ok && maxLength > 0
}

// Truncate limits text by Unicode characters and prefers a nearby word boundary.
func Truncate(text string, maxLength int) string {
	text = strings.TrimSpace(text)
	if maxLength <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxLength {
		return text
	}

	truncated := runes[:maxLength]
	minimumBoundary := maxLength - 50
	if minimumBoundary < 0 {
		minimumBoundary = 0
	}
	for i := len(truncated) - 1; i > minimumBoundary; i-- {
		if truncated[i] == ' ' {
			truncated = truncated[:i]
			break
		}
	}
	return strings.TrimSpace(string(truncated))
}
