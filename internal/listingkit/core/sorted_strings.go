package core

import (
	"slices"
	"strings"
)

// SortedUniqueStrings returns a sorted slice of unique non-empty strings.
func SortedUniqueStrings(values []string) []string {
	out := uniqueStrings(values)
	if len(out) == 0 {
		return nil
	}
	slices.Sort(out)
	return out
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
