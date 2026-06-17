package listingkit

import sheinworkspace "task-processor/internal/listingkit/workspace/shein"

func buildRevisionApplyResult(req *ApplyRevisionRequest, listingResult *ListingKitResult, appliedChanges *RevisionDiffPreview) *RevisionApplyResult {
	if req == nil || listingResult == nil {
		return nil
	}

	actionType := revisionActionType(req)
	headline := "更新 SHEIN 资料"
	if actionType == RevisionActionTypeRestore {
		headline = "恢复历史版本"
	}
	changeCount := 0
	if appliedChanges != nil {
		changeCount = appliedChanges.ChangeCount
	}
	nextActions := buildRevisionSuccessNextActions(listingResult)
	readinessProjection := buildRevisionSuccessReadinessProjection(listingResult)
	statusSummary := buildRevisionSuccessStatusSummaryFromProjection(listingResult, readinessProjection)
	messages := buildRevisionSuccessMessages(revisionSuccessModeApply, headline, changeCount, "", statusSummary)
	recommendedView := buildRevisionSuccessRecommendedView(revisionSuccessModeApply, listingResult, statusSummary)
	applyFollowUp := buildRevisionSuccessFollowUpData(
		revisionSuccessModeApply,
		listingResult,
		statusSummary,
		messages,
		nextActions,
		readinessProjection,
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
