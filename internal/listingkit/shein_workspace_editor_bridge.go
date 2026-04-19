package listingkit

import sheinworkspace "task-processor/internal/workspace/shein"

type SheinEditorRecommendationMeta = sheinworkspace.EditorRecommendationMeta
type SheinEditorAttributeSuggestion = sheinworkspace.EditorAttributeSuggestion
type SheinEditorSaleCandidateSuggestion = sheinworkspace.EditorSaleCandidateSuggestion
type SheinEditorEffect = sheinworkspace.EditorEffect
type SheinEditorProgress = sheinworkspace.EditorProgress
type SheinEditorProgressSection = sheinworkspace.EditorProgressSection
type SheinEditorDirtyHints = sheinworkspace.EditorDirtyHints
type SheinEditorDirtyHintSection = sheinworkspace.EditorDirtyHintSection

func buildSheinCategoryRecommendationMeta(pkg *SheinPackage) *SheinEditorRecommendationMeta {
	return sheinworkspace.BuildCategoryRecommendationMeta(pkg)
}

func buildSheinAttributeRecommendationMeta(pkg *SheinPackage) *SheinEditorRecommendationMeta {
	return sheinworkspace.BuildAttributeRecommendationMeta(pkg)
}

func buildSheinSaleRecommendationMeta(pkg *SheinPackage) *SheinEditorRecommendationMeta {
	return sheinworkspace.BuildSaleRecommendationMeta(pkg)
}

func buildSheinAttributeSuggestions(pkg *SheinPackage) []SheinEditorAttributeSuggestion {
	return sheinworkspace.BuildAttributeSuggestions(pkg)
}

func buildSheinSaleCandidateSuggestions(pkg *SheinPackage) []SheinEditorSaleCandidateSuggestion {
	return sheinworkspace.BuildSaleCandidateSuggestions(pkg)
}

func buildSheinCategoryEffects() []SheinEditorEffect {
	return sheinworkspace.BuildCategoryEffects()
}

func buildSheinAttributeEffects() []SheinEditorEffect {
	return sheinworkspace.BuildAttributeEffects()
}

func buildSheinSaleAttributeEffects() []SheinEditorEffect {
	return sheinworkspace.BuildSaleAttributeEffects()
}

func buildSheinEditorProgress(pkg *SheinPackage, checklist *SheinSubmitChecklist) *SheinEditorProgress {
	checklistTotal := 0
	if checklist != nil {
		checklistTotal = len(checklist.Required) + len(checklist.Recommended) + len(checklist.Optional)
	}
	return sheinworkspace.BuildEditorProgress(pkg, checklistTotal)
}

func buildSheinEditorDirtyHints(pkg *SheinPackage) *SheinEditorDirtyHints {
	return sheinworkspace.BuildEditorDirtyHints(pkg)
}
