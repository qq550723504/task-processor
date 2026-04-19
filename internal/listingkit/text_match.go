package listingkit

import "strings"

func containsText(value string, pattern string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	if value == "" || pattern == "" {
		return false
	}
	return strings.Contains(value, pattern)
}
