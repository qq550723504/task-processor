package shein

import sheinworkspace "task-processor/internal/workspace/shein"

const (
	SuccessModeApply    = sheinworkspace.SuccessModeApply
	SuccessModeRestore  = sheinworkspace.SuccessModeRestore
	SceneApplySuccess   = sheinworkspace.SceneApplySuccess
	SceneRestoreSuccess = sheinworkspace.SceneRestoreSuccess
)

type SuccessStatusSummary = sheinworkspace.SuccessStatusSummary
type SuccessMessages = sheinworkspace.SuccessMessages
type SuccessRecommendedView = sheinworkspace.SuccessRecommendedView
type SuccessFollowUpChecklist[Item any] = sheinworkspace.SuccessFollowUpChecklist[Item]
type SuccessFollowUpOverview = sheinworkspace.SuccessFollowUpOverview
type SuccessSummaryCard = sheinworkspace.SuccessSummaryCard
type SuccessInteractionPresentation = sheinworkspace.SuccessInteractionPresentation
type SuccessCoreData[ChecklistItem any] = sheinworkspace.SuccessCoreData[ChecklistItem]
type SuccessPayload[ChecklistItem any] = sheinworkspace.SuccessPayload[ChecklistItem]

func BuildSuccessNextActions(pkg *Package) []string {
	return sheinworkspace.BuildSuccessNextActions(pkg)
}

func BuildSuccessStatusSummary[Reason any, Hint any](pkg *Package, readiness *SubmitReadiness[Reason, Hint]) *SuccessStatusSummary {
	return sheinworkspace.BuildSuccessStatusSummary(pkg, readiness)
}

func BuildSuccessMessages(mode, headline string, changeCount int, sourceRevisionID string, summary *SuccessStatusSummary) *SuccessMessages {
	return sheinworkspace.BuildSuccessMessages(mode, headline, changeCount, sourceRevisionID, summary)
}

func BuildSuccessRecommendedView(mode string, summary *SuccessStatusSummary) *SuccessRecommendedView {
	return sheinworkspace.BuildSuccessRecommendedView(mode, summary)
}

func BuildSuccessFollowUpChecklist[Reason any, Hint any](checklist *SubmitChecklist[Reason, Hint]) *SuccessFollowUpChecklist[ChecklistGroupItem[Reason, Hint]] {
	return sheinworkspace.BuildSuccessFollowUpChecklist(checklist)
}

func BuildSuccessSuggestedFollowUpRevision(mode string, pkg *Package) *EditorRevisionSkeleton {
	return sheinworkspace.BuildSuccessSuggestedFollowUpRevision(mode, pkg)
}

func BuildSuccessFollowUpOverview[Item any](mode string, summary *SuccessStatusSummary, messages *SuccessMessages, checklist *SuccessFollowUpChecklist[Item], nextActions []string) *SuccessFollowUpOverview {
	return sheinworkspace.BuildSuccessFollowUpOverview(mode, summary, messages, checklist, nextActions)
}

func BuildSuccessSummaryCard(mode, headline, relationText string, changeCount int, messages *SuccessMessages, appliedChanges *RevisionDiffPreview, summary *SuccessStatusSummary, recommendedView *SuccessRecommendedView, nextActions []string) *SuccessSummaryCard {
	return sheinworkspace.BuildSuccessSummaryCard(mode, headline, relationText, changeCount, messages, appliedChanges, summary, recommendedView, nextActions)
}

func BuildSuccessPresentation(scene string, nextActions []string, messages *SuccessMessages, recommendedView *SuccessRecommendedView, summaryCard *SuccessSummaryCard) *SuccessInteractionPresentation {
	return sheinworkspace.BuildSuccessPresentation(scene, nextActions, messages, recommendedView, summaryCard)
}

func BuildSuccessPayload[Item any](mode, actionType, headline, sourceRevisionID, relationText string, changeCount int, statusSummary *SuccessStatusSummary, presentation *SuccessInteractionPresentation, followUpChecklist *SuccessFollowUpChecklist[Item], followUpOverview *SuccessFollowUpOverview, suggestedFollowUpRevision *EditorRevisionSkeleton, appliedChanges *RevisionDiffPreview) *SuccessPayload[Item] {
	return sheinworkspace.BuildSuccessPayload(mode, actionType, headline, sourceRevisionID, relationText, changeCount, statusSummary, presentation, followUpChecklist, followUpOverview, suggestedFollowUpRevision, appliedChanges)
}
