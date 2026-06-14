// Adapter-only bridge. Keep domain rules in internal/marketplace/shein/workspace.
package listingkit

import listingworkspace "task-processor/internal/listingkit/workspace/shein"

type SheinEditorRecommendationMeta = listingworkspace.EditorRecommendationMeta
type SheinEditorAttributeSuggestion = listingworkspace.EditorAttributeSuggestion
type SheinEditorSaleCandidateSuggestion = listingworkspace.EditorSaleCandidateSuggestion
type SheinEditorEffect = listingworkspace.EditorEffect
type SheinEditorProgress = listingworkspace.EditorProgress
type SheinEditorProgressSection = listingworkspace.EditorProgressSection
type SheinEditorDirtyHints = listingworkspace.EditorDirtyHints
type SheinEditorDirtyHintSection = listingworkspace.EditorDirtyHintSection

func buildSheinCategoryRecommendationMeta(pkg *SheinPackage) *SheinEditorRecommendationMeta {
	return listingworkspace.BuildCategoryRecommendationMeta(pkg)
}

func buildSheinAttributeRecommendationMeta(pkg *SheinPackage) *SheinEditorRecommendationMeta {
	return listingworkspace.BuildAttributeRecommendationMeta(pkg)
}

func buildSheinSaleRecommendationMeta(pkg *SheinPackage) *SheinEditorRecommendationMeta {
	return listingworkspace.BuildSaleRecommendationMeta(pkg)
}

func buildSheinAttributeSuggestions(pkg *SheinPackage) []SheinEditorAttributeSuggestion {
	return listingworkspace.BuildAttributeSuggestions(pkg)
}

func buildSheinSaleCandidateSuggestions(pkg *SheinPackage) []SheinEditorSaleCandidateSuggestion {
	return listingworkspace.BuildSaleCandidateSuggestions(pkg)
}

func buildSheinCategoryEffects() []SheinEditorEffect {
	return listingworkspace.BuildCategoryEffects()
}

func buildSheinAttributeEffects() []SheinEditorEffect {
	return listingworkspace.BuildAttributeEffects()
}

func buildSheinSaleAttributeEffects() []SheinEditorEffect {
	return listingworkspace.BuildSaleAttributeEffects()
}

func buildSheinEditorProgress(pkg *SheinPackage, checklist *SheinSubmitChecklist) *SheinEditorProgress {
	return listingworkspace.BuildEditorProgress(pkg, checklist)
}

func buildSheinEditorDirtyHints(pkg *SheinPackage) *SheinEditorDirtyHints {
	return listingworkspace.BuildEditorDirtyHints(pkg)
}
