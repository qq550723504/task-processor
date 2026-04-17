package listingkit

func buildRevisionApplyResult(req *ApplyRevisionRequest, listingResult *ListingKitResult, appliedChanges *RevisionDiffPreview) *RevisionApplyResult {
	if req == nil || listingResult == nil {
		return nil
	}

	actionType := revisionActionType(req)
	headline := buildRevisionApplyHeadline(req)
	changeCount := 0
	if appliedChanges != nil {
		changeCount = appliedChanges.ChangeCount
	}
	nextActions := buildRevisionRestoreResultNextActions(listingResult)
	statusSummary := buildRevisionRestoreStatusSummary(listingResult)
	messages := buildRevisionApplyResultMessages(headline, changeCount, statusSummary)
	recommendedView := buildRevisionApplyRecommendedView(listingResult)
	applyFollowUp := buildRevisionSuccessFollowUpData(
		revisionSuccessModeApply,
		listingResult,
		statusSummary,
		messages,
		nextActions,
	)
	var followUpChecklist *RevisionFollowUpChecklist
	var followUpOverview *RevisionFollowUpOverview
	var suggestedFollowUpRevision *SheinEditorRevisionSkeleton
	if applyFollowUp != nil {
		followUpChecklist = applyFollowUp.Checklist
		followUpOverview = applyFollowUp.Overview
		suggestedFollowUpRevision = applyFollowUp.SuggestedRevision
	}
	summaryCard := buildRevisionApplySummaryCard(
		headline,
		changeCount,
		messages,
		appliedChanges,
		listingResult,
	)
	presentation := buildRevisionInteractionPresentation(
		revisionPresentationSceneApplySuccess,
		nextActions,
		messages,
		recommendedView,
		summaryCard,
	)

	return &RevisionApplyResult{
		Applied: true,
		SuccessPayload: buildRevisionSuccessPayloadForApply(
			actionType,
			headline,
			changeCount,
			statusSummary,
			presentation,
			followUpChecklist,
			followUpOverview,
			suggestedFollowUpRevision,
			appliedChanges,
		),
	}
}

func buildRevisionApplyHeadline(req *ApplyRevisionRequest) string {
	if req == nil {
		return ""
	}
	if revisionActionType(req) == RevisionActionTypeRestore {
		return "恢复历史版本"
	}
	return "更新 SHEIN 资料"
}
