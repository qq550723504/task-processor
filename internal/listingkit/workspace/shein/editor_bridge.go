package shein

import (
	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
)

func BuildCategoryRecommendationMeta(pkg *Package) *EditorRecommendationMeta {
	return sheinmarketplace.BuildCategoryRecommendationMeta(pkg)
}

func BuildAttributeRecommendationMeta(pkg *Package) *EditorRecommendationMeta {
	return sheinmarketplace.BuildAttributeRecommendationMeta(pkg)
}

func BuildSaleRecommendationMeta(pkg *Package) *EditorRecommendationMeta {
	return sheinmarketplace.BuildSaleRecommendationMeta(pkg)
}

func BuildAttributeSuggestions(pkg *Package) []EditorAttributeSuggestion {
	return sheinmarketplace.BuildAttributeSuggestions(pkg)
}

func BuildSaleCandidateSuggestions(pkg *Package) []EditorSaleCandidateSuggestion {
	return sheinmarketplace.BuildSaleCandidateSuggestions(pkg)
}

func BuildCategoryEffects() []EditorEffect {
	return sheinmarketplace.BuildCategoryEffects()
}

func BuildAttributeEffects() []EditorEffect {
	return sheinmarketplace.BuildAttributeEffects()
}

func BuildSaleAttributeEffects() []EditorEffect {
	return sheinmarketplace.BuildSaleAttributeEffects()
}

func BuildEditorProgress[R any, H any](pkg *Package, checklist *SubmitChecklist[R, H]) *EditorProgress {
	return sheinmarketplace.BuildEditorProgress(pkg, sheinmarketplace.ChecklistItemCount(checklist))
}

func BuildEditorDirtyHints(pkg *Package) *EditorDirtyHints {
	return sheinmarketplace.BuildEditorDirtyHints(pkg)
}
