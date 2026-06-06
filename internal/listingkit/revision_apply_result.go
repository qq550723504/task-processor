package listingkit

import sheinworkspace "task-processor/internal/listingkit/workspace/shein"

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
	nextActions := buildRevisionSuccessNextActions(listingResult)
	statusSummary := buildRevisionSuccessStatusSummary(listingResult)
	messages := buildRevisionSuccessMessages(revisionSuccessModeApply, headline, changeCount, "", statusSummary)
	recommendedView := buildRevisionSuccessRecommendedView(revisionSuccessModeApply, listingResult, statusSummary)
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
	summaryCard := sheinworkspace.BuildSuccessSummaryCard(
		sheinworkspace.SuccessModeApply,
		headline,
		"",
		changeCount,
		messages,
		appliedChanges,
		statusSummary,
		recommendedView,
		nextActions,
	)
	presentation := sheinworkspace.BuildSuccessPresentation(
		sheinworkspace.SceneApplySuccess,
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
