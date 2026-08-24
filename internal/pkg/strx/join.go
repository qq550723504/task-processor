package strx

import "strings"

// JoinNonBlank trims values, drops blank entries, and joins the remaining
// values with sep. Duplicate non-blank values are intentionally preserved.
func JoinNonBlank(values []string, sep string) string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			filtered = append(filtered, value)
		}
	}
	return strings.Join(filtered, sep)
}
