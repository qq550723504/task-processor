package shein

import (
	"strings"

	"task-processor/internal/catalog/canonical"
)

func buildCategoryFamilyConflictSummary(canonical *canonical.Product, pkg *Package) (bool, string) {
	if pkg != nil && pkg.CategoryResolution != nil && semanticRejectsCategory(pkg.CategoryResolution.SemanticValidation) {
		reason := strings.TrimSpace(pkg.CategoryResolution.SemanticValidation.Reason)
		if reason == "" {
			reason = "当前类目路径与商品语义明显不一致，建议优先人工复核 SHEIN 类目是否正确"
		}
		return true, reason
	}
	return false, ""
}

func buildCategoryFamilyConflictReviewNotes(canonical *canonical.Product, pkg *Package) []string {
	recommend, reason := buildCategoryFamilyConflictSummary(canonical, pkg)
	if !recommend {
		return nil
	}
	currentPath := currentCategoryPath(canonical, pkg)
	return []string{
		reason + "（当前类目: " + strings.Join(currentPath, " > ") + "）",
	}
}
