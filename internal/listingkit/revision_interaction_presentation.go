package listingkit

import sheinworkspace "task-processor/internal/listingkit/workspace/shein"

const (
	revisionPresentationSceneApplySuccess   = "apply_success"
	revisionPresentationSceneRestoreSuccess = "restore_success"
	revisionPresentationSceneRestorePreview = "restore_preview"
)

func buildRevisionInteractionPresentation(
	scene string,
	nextActions []string,
	messages *RevisionResultMessages,
	recommendedView *RevisionRecommendedView,
	summaryCard *RevisionSuccessSummaryCard,
) *RevisionInteractionPresentation {
	return sheinworkspace.BuildSuccessPresentation(scene, nextActions, messages, recommendedView, summaryCard)
}
