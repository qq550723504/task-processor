package listingkit

import sheinworkspace "task-processor/internal/workspace/shein"

func buildRevisionRestoreResult(req *ApplyRevisionRequest, listingResult *ListingKitResult, appliedChanges *RevisionDiffPreview) *RevisionRestoreResult {
	sourceRevisionID := revisionRestoreSourceID(req)
	if sourceRevisionID == "" {
		return nil
	}

	record := ListingKitRevisionRecord{
		ActionType:             revisionActionType(req),
		RestoredFromRevisionID: sourceRevisionID,
		AppliedChanges:         appliedChanges,
	}
	record = withRevisionTimelineSummary(record)

	headline := ""
	relationText := ""
	restoredFieldCount := 0
	if record.Timeline != nil {
		headline = record.Timeline.Headline
		relationText = record.Timeline.RelationText
		restoredFieldCount = record.Timeline.ChangeCount
	}
	nextActions := buildRevisionSuccessNextActions(listingResult)
	statusSummary := buildRevisionSuccessStatusSummary(listingResult)
	messages := buildRevisionSuccessMessages(revisionSuccessModeRestore, headline, restoredFieldCount, sourceRevisionID, statusSummary)
	recommendedView := buildRevisionSuccessRecommendedView(revisionSuccessModeRestore, listingResult, statusSummary)
	restoreFollowUp := buildRevisionSuccessFollowUpData(
		revisionSuccessModeRestore,
		listingResult,
		statusSummary,
		messages,
		nextActions,
	)
	var followUpChecklist *RevisionFollowUpChecklist
	var followUpOverview *RevisionFollowUpOverview
	var suggestedFollowUpRevision *SheinEditorRevisionSkeleton
	if restoreFollowUp != nil {
		followUpChecklist = restoreFollowUp.Checklist
		followUpOverview = restoreFollowUp.Overview
		suggestedFollowUpRevision = restoreFollowUp.SuggestedRevision
	}
	summaryCard := sheinworkspace.BuildSuccessSummaryCard(
		sheinworkspace.SuccessModeRestore,
		headline,
		relationText,
		restoredFieldCount,
		messages,
		appliedChanges,
		statusSummary,
		recommendedView,
		nextActions,
	)
	presentation := sheinworkspace.BuildSuccessPresentation(
		sheinworkspace.SceneRestoreSuccess,
		nextActions,
		messages,
		recommendedView,
		summaryCard,
	)

	return &RevisionRestoreResult{
		Applied: true,
		SuccessPayload: buildRevisionSuccessPayloadForRestore(
			record.ActionType,
			headline,
			sourceRevisionID,
			relationText,
			restoredFieldCount,
			statusSummary,
			presentation,
			followUpChecklist,
			followUpOverview,
			suggestedFollowUpRevision,
			appliedChanges,
		),
	}
}
