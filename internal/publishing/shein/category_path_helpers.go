package shein

import (
	"strings"

	"task-processor/internal/productenrich"
)

func currentCategoryPath(canonical *productenrich.CanonicalProduct, current *Package) []string {
	if current != nil && current.CategoryResolution != nil && len(current.CategoryResolution.MatchedPath) > 0 {
		return append([]string(nil), current.CategoryResolution.MatchedPath...)
	}
	if current != nil && len(current.CategoryPath) > 0 {
		return append([]string(nil), current.CategoryPath...)
	}
	if canonical != nil && len(canonical.CategoryPath) > 0 {
		return append([]string(nil), canonical.CategoryPath...)
	}
	return nil
}

func normalizeCategoryToken(in string) string {
	replacer := strings.NewReplacer("&", " ", "/", " ", "-", " ", "_", " ", ">", " ")
	token := strings.ToLower(strings.TrimSpace(replacer.Replace(in)))
	token = strings.Join(strings.Fields(token), " ")
	return token
}
