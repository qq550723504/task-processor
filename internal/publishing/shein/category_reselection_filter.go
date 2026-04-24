package shein

import (
	"strings"

	"task-processor/internal/productenrich"
)

func shouldAcceptSuggestedCategory(canonical *productenrich.CanonicalProduct, current *Package, suggested *CategorySuggestion) bool {
	return shouldAcceptSuggestedCategoryWithSemanticVerifier(canonical, current, suggested, nil)
}

func shouldAcceptSuggestedCategoryWithSemanticVerifier(canonical *productenrich.CanonicalProduct, current *Package, suggested *CategorySuggestion, semanticVerifier categorySemanticVerifier) bool {
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

	if len(currentPath) > 0 && sharedPathTokenCount(currentPath, suggested.MatchedPath) >= 2 {
		return true
	}

	if semanticVerifier != nil {
		validation := semanticVerifier.ValidateProductCategory(canonical, current, suggested.MatchedPath)
		switch strings.ToLower(strings.TrimSpace(validationVerdict(validation))) {
		case "incompatible":
			return false
		case "compatible":
			return true
		}
	}

	fit := evaluateSuggestedCategoryFit(currentPath, productTokens, suggested.MatchedPath)
	return fit.Score >= 3
}

func validationVerdict(validation *CategorySemanticValidation) string {
	if validation == nil {
		return ""
	}
	return validation.Verdict
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
