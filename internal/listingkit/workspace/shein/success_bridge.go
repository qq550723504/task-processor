package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

const (
	SuccessModeApply    = sheinmarketplace.SuccessModeApply
	SuccessModeRestore  = sheinmarketplace.SuccessModeRestore
	SceneApplySuccess   = sheinmarketplace.SceneApplySuccess
	SceneRestoreSuccess = sheinmarketplace.SceneRestoreSuccess
)

type SuccessStatusSummary = sheinmarketplace.SuccessStatusSummary
type SuccessMessages = sheinmarketplace.SuccessMessages
type SuccessRecommendedView = sheinmarketplace.SuccessRecommendedView
type SuccessFollowUpChecklist[Item any] = sheinmarketplace.SuccessFollowUpChecklist[Item]
type SuccessFollowUpOverview = sheinmarketplace.SuccessFollowUpOverview
type SuccessSummaryCard = sheinmarketplace.SuccessSummaryCard
type SuccessInteractionPresentation = sheinmarketplace.SuccessInteractionPresentation
type SuccessCoreData[ChecklistItem any] = sheinmarketplace.SuccessCoreData[ChecklistItem]
type SuccessPayload[ChecklistItem any] = sheinmarketplace.SuccessPayload[ChecklistItem]

func BuildSuccessNextActions(pkg *Package) []string {
	return sheinmarketplace.BuildSuccessNextActions(pkg)
}

func BuildSuccessStatusSummary[Reason any, Hint any](pkg *Package, readiness *SubmitReadiness[Reason, Hint]) *SuccessStatusSummary {
	return sheinmarketplace.BuildSuccessStatusSummary(pkg, readiness)
}

func BuildSuccessMessages(mode, headline string, changeCount int, sourceRevisionID string, summary *SuccessStatusSummary) *SuccessMessages {
	return sheinmarketplace.BuildSuccessMessages(mode, headline, changeCount, sourceRevisionID, summary)
}

func BuildSuccessRecommendedView(mode string, summary *SuccessStatusSummary) *SuccessRecommendedView {
	return sheinmarketplace.BuildSuccessRecommendedView(mode, summary)
}

func BuildSuccessFollowUpChecklist[Reason any, Hint any](checklist *SubmitChecklist[Reason, Hint]) *SuccessFollowUpChecklist[ChecklistGroupItem[Reason, Hint]] {
	return sheinmarketplace.BuildSuccessFollowUpChecklist(checklist)
}

func BuildSuccessSuggestedFollowUpRevision(mode string, pkg *Package) *EditorRevisionSkeleton {
	return sheinmarketplace.BuildSuccessSuggestedFollowUpRevision(mode, pkg)
}

func BuildSuccessFollowUpOverview[Item any](mode string, summary *SuccessStatusSummary, messages *SuccessMessages, checklist *SuccessFollowUpChecklist[Item], nextActions []string) *SuccessFollowUpOverview {
	return sheinmarketplace.BuildSuccessFollowUpOverview(mode, summary, messages, checklist, nextActions)
}

func BuildSuccessSummaryCard(mode, headline, relationText string, changeCount int, messages *SuccessMessages, appliedChanges *RevisionDiffPreview, summary *SuccessStatusSummary, recommendedView *SuccessRecommendedView, nextActions []string) *SuccessSummaryCard {
	return sheinmarketplace.BuildSuccessSummaryCard(mode, headline, relationText, changeCount, messages, appliedChanges, summary, recommendedView, nextActions)
}

func BuildSuccessPresentation(scene string, nextActions []string, messages *SuccessMessages, recommendedView *SuccessRecommendedView, summaryCard *SuccessSummaryCard) *SuccessInteractionPresentation {
	return sheinmarketplace.BuildSuccessPresentation(scene, nextActions, messages, recommendedView, summaryCard)
}

func BuildSuccessPayload[Item any](mode, actionType, headline, sourceRevisionID, relationText string, changeCount int, statusSummary *SuccessStatusSummary, presentation *SuccessInteractionPresentation, followUpChecklist *SuccessFollowUpChecklist[Item], followUpOverview *SuccessFollowUpOverview, suggestedFollowUpRevision *EditorRevisionSkeleton, appliedChanges *RevisionDiffPreview) *SuccessPayload[Item] {
	return sheinmarketplace.BuildSuccessPayload(mode, actionType, headline, sourceRevisionID, relationText, changeCount, statusSummary, presentation, followUpChecklist, followUpOverview, suggestedFollowUpRevision, appliedChanges)
}
