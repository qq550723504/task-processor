package listingkit

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
	nextActions := buildRevisionRestoreResultNextActions(listingResult)
	statusSummary := buildRevisionRestoreStatusSummary(listingResult)
	messages := buildRevisionRestoreResultMessages(headline, restoredFieldCount, sourceRevisionID, statusSummary)
	recommendedView := buildRevisionRestoreRecommendedView(listingResult, statusSummary)
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
	summaryCard := buildRevisionRestoreSummaryCard(
		headline,
		relationText,
		nextActions,
		statusSummary,
		messages,
		recommendedView,
	)
	presentation := buildRevisionInteractionPresentation(
		revisionPresentationSceneRestoreSuccess,
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
