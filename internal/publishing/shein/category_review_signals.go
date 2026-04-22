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
	productFamilies := productFamilyLabels(canonical, pkg)
	currentFamilies := familyLabelsForTokens(normalizedCategoryTokens(currentPath))
	if len(productFamilies) == 0 || len(currentFamilies) == 0 {
		return false, ""
	}
	if !likelyTokenFamilyConflict(productFamilies, currentFamilies) {
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
