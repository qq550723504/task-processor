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
	productTokens := productSignalTokens(canonical, pkg)
	if len(productTokens) == 0 {
		return false, ""
	}
	if evaluateSuggestedCategoryFit(nil, productTokens, currentPath).Score >= 2 {
		return false, ""
	}
	return true, "当前类目路径与商品语义明显不一致，建议优先人工复核 SHEIN 类目是否正确"
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
