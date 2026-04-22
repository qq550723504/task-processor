package shein

import (
	"strings"

	"task-processor/internal/productenrich"
)

func shouldAcceptSuggestedCategory(canonical *productenrich.CanonicalProduct, current *Package, suggested *CategorySuggestion) bool {
	if suggested == nil || suggested.CategoryID <= 0 {
		return false
	}
	if current != nil && current.CategoryID > 0 && current.CategoryID == suggested.CategoryID {
		return false
	}
	if len(suggested.MatchedPath) == 0 {
		return false
	}

	currentPath := currentCategoryPath(canonical, current)
	productTokens := productSignalTokens(canonical, current)
	productFamilies := familyLabelsForTokens(productTokens)
	suggestedTokens := normalizedCategoryTokens(suggested.MatchedPath)
	suggestedFamilies := familyLabelsForTokens(suggestedTokens)

	if len(currentPath) > 0 && sharedPathTokenCount(currentPath, suggested.MatchedPath) >= 2 {
		return true
	}

	if likelyProductFamilyConflict(currentPath, suggested.MatchedPath) {
		return false
	}

	if likelyTokenFamilyConflict(productFamilies, suggestedFamilies) {
		return false
	}

	fit := evaluateSuggestedCategoryFit(currentPath, productTokens, productFamilies, suggested.MatchedPath)
	return fit.Score >= 3
}

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

func sharedPathTokenCount(a, b []string) int {
	set := map[string]struct{}{}
	for _, item := range a {
		token := normalizeCategoryToken(item)
		if token != "" {
			set[token] = struct{}{}
		}
	}
	count := 0
	for _, item := range b {
		token := normalizeCategoryToken(item)
		if token == "" {
			continue
		}
		if _, ok := set[token]; ok {
			count++
		}
	}
	return count
}

func likelyProductFamilyConflict(currentPath, suggestedPath []string) bool {
	currentTokens := normalizedCategoryTokens(currentPath)
	suggestedTokens := normalizedCategoryTokens(suggestedPath)

	if intersectsAny(currentTokens, []string{"camping", "outdoor", "furniture", "chair", "table", "露营", "户外", "家具", "椅", "桌"}) &&
		intersectsAny(suggestedTokens, []string{"clothing", "apparel", "dress", "costume", "lolita", "服装", "服饰", "裙", "洛丽塔"}) {
		return true
	}

	if intersectsAny(currentTokens, []string{"kitchen", "cup", "bottle", "drinkware", "厨具", "杯", "水壶"}) &&
		intersectsAny(suggestedTokens, []string{"phone", "electronics", "mobile", "手机", "电子"}) {
		return true
	}

	return false
}

func likelyTokenFamilyConflict(productFamilies, suggestedFamilies []string) bool {
	if len(productFamilies) == 0 || len(suggestedFamilies) == 0 {
		return false
	}
	if sharedFamilyLabelCount(productFamilies, suggestedFamilies) > 0 {
		return false
	}

	conflicts := map[string][]string{
		"outdoor_furniture": {"apparel", "electronics", "footwear"},
		"drinkware":         {"apparel", "electronics", "footwear"},
		"apparel":           {"outdoor_furniture", "drinkware", "electronics"},
		"electronics":       {"drinkware", "apparel", "footwear"},
		"footwear":          {"drinkware", "electronics", "outdoor_furniture"},
	}

	for _, family := range productFamilies {
		for _, conflict := range conflicts[family] {
			for _, suggestedFamily := range suggestedFamilies {
				if suggestedFamily == conflict {
					return true
				}
			}
		}
	}

	return false
}

func normalizedCategoryTokens(path []string) []string {
	out := make([]string, 0, len(path))
	for _, item := range path {
		if token := normalizeCategoryToken(item); token != "" {
			out = append(out, token)
		}
	}
	return out
}

func normalizeCategoryToken(in string) string {
	replacer := strings.NewReplacer("&", " ", "/", " ", "-", " ", "_", " ", ">", " ")
	token := strings.ToLower(strings.TrimSpace(replacer.Replace(in)))
	token = strings.Join(strings.Fields(token), " ")
	return token
}

func sharedSignalTokenCount(a, b []string) int {
	set := map[string]struct{}{}
	for _, token := range a {
		set[token] = struct{}{}
	}
	count := 0
	for _, token := range b {
		if _, ok := set[token]; ok {
			count++
		}
	}
	return count
}

func intersectsAny(tokens []string, candidates []string) bool {
	for _, token := range tokens {
		for _, candidate := range candidates {
			if strings.Contains(token, candidate) {
				return true
			}
		}
	}
	return false
}
