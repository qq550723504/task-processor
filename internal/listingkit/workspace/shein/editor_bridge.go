package shein

import sheinworkspace "task-processor/internal/workspace/shein"

func BuildCategoryRecommendationMeta(pkg *Package) *EditorRecommendationMeta {
	return sheinworkspace.BuildCategoryRecommendationMeta(pkg)
}

func BuildAttributeRecommendationMeta(pkg *Package) *EditorRecommendationMeta {
	return sheinworkspace.BuildAttributeRecommendationMeta(pkg)
}

func BuildSaleRecommendationMeta(pkg *Package) *EditorRecommendationMeta {
	return sheinworkspace.BuildSaleRecommendationMeta(pkg)
}

func BuildAttributeSuggestions(pkg *Package) []EditorAttributeSuggestion {
	return sheinworkspace.BuildAttributeSuggestions(pkg)
}

func BuildSaleCandidateSuggestions(pkg *Package) []EditorSaleCandidateSuggestion {
	return sheinworkspace.BuildSaleCandidateSuggestions(pkg)
}

func BuildCategoryEffects() []EditorEffect {
	return sheinworkspace.BuildCategoryEffects()
}

func BuildAttributeEffects() []EditorEffect {
	return sheinworkspace.BuildAttributeEffects()
}

func BuildSaleAttributeEffects() []EditorEffect {
	return sheinworkspace.BuildSaleAttributeEffects()
}

func BuildEditorProgress[R any, H any](pkg *Package, checklist *SubmitChecklist[R, H]) *EditorProgress {
	return sheinworkspace.BuildEditorProgress(pkg, sheinworkspace.ChecklistItemCount(checklist))
}

func BuildEditorDirtyHints(pkg *Package) *EditorDirtyHints {
	return sheinworkspace.BuildEditorDirtyHints(pkg)
}
