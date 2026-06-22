// Adapter-only bridge. Keep domain rules in internal/marketplace/shein/workspace.
package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

type SheinEditorRecommendationMeta = sheinworkspace.EditorRecommendationMeta
type SheinEditorAttributeSuggestion = sheinworkspace.EditorAttributeSuggestion
type SheinEditorSaleCandidateSuggestion = sheinworkspace.EditorSaleCandidateSuggestion
type SheinEditorEffect = sheinworkspace.EditorEffect
type SheinEditorProgress = sheinworkspace.EditorProgress
type SheinEditorProgressSection = sheinworkspace.EditorProgressSection
type SheinEditorDirtyHints = sheinworkspace.EditorDirtyHints
type SheinEditorDirtyHintSection = sheinworkspace.EditorDirtyHintSection

func buildSheinEditorProgress(pkg *SheinPackage, checklist *SheinSubmitChecklist) *SheinEditorProgress {
	return sheinworkspace.BuildEditorProgress(pkg, sheinworkspace.ChecklistItemCount(checklist))
}
