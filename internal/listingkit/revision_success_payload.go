package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

func buildRevisionSuccessPayloadForApply(
	actionType string,
	headline string,
	changeCount int,
	statusSummary *RevisionStatusSummary,
	presentation *RevisionInteractionPresentation,
	followUpChecklist *RevisionFollowUpChecklist,
	followUpOverview *RevisionFollowUpOverview,
	suggestedFollowUpRevision *SheinEditorRevisionSkeleton,
	appliedChanges *RevisionDiffPreview,
) *RevisionSuccessPayload {
	return sheinworkspace.BuildSuccessPayload(
		string(revisionSuccessModeApply),
		actionType,
		headline,
		"",
		"",
		changeCount,
		statusSummary,
		presentation,
		followUpChecklist,
		followUpOverview,
		suggestedFollowUpRevision,
		appliedChanges,
	)
}

func buildRevisionSuccessPayloadForRestore(
	actionType string,
	headline string,
	sourceRevisionID string,
	relationText string,
	changeCount int,
	statusSummary *RevisionStatusSummary,
	presentation *RevisionInteractionPresentation,
	followUpChecklist *RevisionFollowUpChecklist,
	followUpOverview *RevisionFollowUpOverview,
	suggestedFollowUpRevision *SheinEditorRevisionSkeleton,
	appliedChanges *RevisionDiffPreview,
) *RevisionSuccessPayload {
	return sheinworkspace.BuildSuccessPayload(
		string(revisionSuccessModeRestore),
		actionType,
		headline,
		sourceRevisionID,
		relationText,
		changeCount,
		statusSummary,
		presentation,
		followUpChecklist,
		followUpOverview,
		suggestedFollowUpRevision,
		appliedChanges,
	)
}
