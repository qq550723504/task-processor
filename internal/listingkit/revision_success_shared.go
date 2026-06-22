package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

type revisionSuccessMode string

const (
	revisionSuccessModeApply   revisionSuccessMode = "apply"
	revisionSuccessModeRestore revisionSuccessMode = "restore"
)

type revisionSuccessFollowUpData struct {
	Checklist         *RevisionFollowUpChecklist
	Overview          *RevisionFollowUpOverview
	SuggestedRevision *SheinEditorRevisionSkeleton
}

func buildRevisionSuccessNextActions(result *ListingKitResult) []string {
	if result == nil {
		return nil
	}
	return sheinworkspace.BuildSuccessNextActions(result.Shein)
}

func buildRevisionSuccessStatusSummary(result *ListingKitResult) *RevisionStatusSummary {
	return buildRevisionSuccessStatusSummaryFromProjection(result, buildRevisionSuccessReadinessProjection(result))
}

func buildRevisionSuccessReadinessProjection(result *ListingKitResult) *sheinSubmitReadinessProjection {
	if result == nil || result.Shein == nil {
		return nil
	}
	return buildSheinSubmitReadinessProjectionWithPod(result.Shein, result.PodExecution)
}

func buildRevisionSuccessStatusSummaryFromProjection(result *ListingKitResult, projection *sheinSubmitReadinessProjection) *RevisionStatusSummary {
	if result == nil || result.Shein == nil {
		return nil
	}
	if projection == nil {
		return nil
	}
	return sheinworkspace.BuildSuccessStatusSummary(result.Shein, projection.Readiness)
}

func buildRevisionSuccessMessages(mode revisionSuccessMode, headline string, changeCount int, sourceRevisionID string, summary *RevisionStatusSummary) *RevisionResultMessages {
	return sheinworkspace.BuildSuccessMessages(string(mode), headline, changeCount, sourceRevisionID, summary)
}

func buildRevisionSuccessRecommendedView(mode revisionSuccessMode, result *ListingKitResult, summary *RevisionStatusSummary) *RevisionRecommendedView {
	_ = result
	return sheinworkspace.BuildSuccessRecommendedView(string(mode), summary)
}

func buildRevisionSuccessFollowUpChecklist(result *ListingKitResult) *RevisionFollowUpChecklist {
	return buildRevisionSuccessFollowUpChecklistFromProjection(buildRevisionSuccessReadinessProjection(result))
}

func buildRevisionSuccessFollowUpChecklistFromProjection(projection *sheinSubmitReadinessProjection) *RevisionFollowUpChecklist {
	if projection == nil {
		return nil
	}
	return sheinworkspace.BuildSuccessFollowUpChecklist(projection.Checklist)
}

func buildRevisionSuccessSuggestedFollowUpRevision(mode revisionSuccessMode, result *ListingKitResult) *SheinEditorRevisionSkeleton {
	if result == nil || result.Shein == nil {
		return nil
	}
	return sheinworkspace.BuildSuccessSuggestedFollowUpRevision(string(mode), result.Shein)
}

func buildRevisionSuccessFollowUpData(
	mode revisionSuccessMode,
	result *ListingKitResult,
	summary *RevisionStatusSummary,
	messages *RevisionResultMessages,
	nextActions []string,
	readinessProjection *sheinSubmitReadinessProjection,
) *revisionSuccessFollowUpData {
	checklist := buildRevisionSuccessFollowUpChecklistFromProjection(readinessProjection)
	overview := buildRevisionSuccessFollowUpOverview(mode, summary, messages, checklist, nextActions)
	suggested := buildRevisionSuccessSuggestedFollowUpRevision(mode, result)
	if checklist == nil && overview == nil && suggested == nil {
		return nil
	}
	return &revisionSuccessFollowUpData{
		Checklist:         checklist,
		Overview:          overview,
		SuggestedRevision: suggested,
	}
}

func buildRevisionSuccessFollowUpOverview(mode revisionSuccessMode, summary *RevisionStatusSummary, messages *RevisionResultMessages, checklist *RevisionFollowUpChecklist, nextActions []string) *RevisionFollowUpOverview {
	return sheinworkspace.BuildSuccessFollowUpOverview(string(mode), summary, messages, checklist, nextActions)
}
