package shein

import (
	"strings"

	"task-processor/internal/productenrich"
)

func buildCategoryFamilyConflictSummary(canonical *productenrich.CanonicalProduct, pkg *Package) (bool, string) {
	currentPath := currentCategoryPath(canonical, pkg)
	if len(currentPath) == 0 {
		return false, ""
	}
	if pkg != nil && pkg.CategoryResolution != nil && semanticRejectsCategory(pkg.CategoryResolution.SemanticValidation) {
		reason := strings.TrimSpace(pkg.CategoryResolution.SemanticValidation.Reason)
		if reason == "" {
			reason = "当前类目路径与商品语义明显不一致，建议优先人工复核 SHEIN 类目是否正确"
		}
		return true, reason
	}
	if pkg != nil && pkg.CategoryResolution != nil && semanticAcceptsCategory(pkg.CategoryResolution.SemanticValidation) {
		return false, ""
	}
	productTokens := productSignalTokens(canonical, pkg)
	if len(productTokens) == 0 {
		return false, ""
	}
	if categoryLeafMatchesProductSignals(currentPath, productTokens) {
		return false, ""
	}
	if evaluateSuggestedCategoryFit(nil, productTokens, currentPath).Score >= 2 {
		return false, ""
	}
	return true, "当前类目路径与商品语义明显不一致，建议优先人工复核 SHEIN 类目是否正确"
}

func categoryLeafMatchesProductSignals(path []string, productTokens []string) bool {
	if len(path) == 0 || len(productTokens) == 0 {
		return false
	}
	leaf := normalizeCategoryToken(path[len(path)-1])
	if leaf == "" {
		return false
	}
	for _, token := range productTokens {
		if token == leaf || strings.Contains(token, leaf) || strings.Contains(leaf, token) {
			return true
		}
	}
	return false
}

func buildCategoryFamilyConflictReviewNotes(canonical *productenrich.CanonicalProduct, pkg *Package) []string {
	recommend, reason := buildCategoryFamilyConflictSummary(canonical, pkg)
	if !recommend {
		return nil
	}
	currentPath := currentCategoryPath(canonical, pkg)
	return []string{
		reason + "（当前类目: " + strings.Join(currentPath, " > ") + "）",
	}
}
