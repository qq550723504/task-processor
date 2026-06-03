package listingkit

import (
	"strings"
)

func shouldStopGenerationNavigationDispatchPlan(plan *GenerationNavigationDispatchPlan, step *GenerationNavigationDispatchExecutionStep) bool {
	if plan == nil || step == nil {
		return false
	}
	if plan.StopOnNotModified && step.NotModified {
		return true
	}
	if plan.StopOnFirstSuccess && step.Executed && !step.NotModified {
		return true
	}
	return false
}

func generationNavigationDispatchPlanStopReason(plan *GenerationNavigationDispatchPlan, step *GenerationNavigationDispatchExecutionStep) string {
	if plan == nil || step == nil {
		return ""
	}
	if plan.StopOnNotModified && step.NotModified {
		return "not_modified"
	}
	if plan.StopOnFirstSuccess && step.Executed && !step.NotModified {
		return "first_success"
	}
	return ""
}

func generationNavigationDispatchPlanRunsInParallel(plan *GenerationNavigationDispatchPlan) bool {
	if plan == nil {
		return false
	}
	return !plan.StopOnError && !plan.StopOnFirstSuccess && !plan.StopOnNotModified && len(plan.Steps) > 1
}

func generationNavigationDispatchStepDeduplicationKey(step GenerationNavigationDispatchStep, responseMode string) string {
	query := cloneGenerationQueueQuery(step.Query)
	if query != nil && strings.TrimSpace(query.ResponseMode) == "" {
		query.ResponseMode = firstNonEmpty(step.ResponseMode, responseMode)
	}
	return hashRenderRevision(
		strings.ToLower(strings.TrimSpace(step.Kind)),
		firstNonEmpty(step.ResponseMode, responseMode),
		firstNonEmpty(step.CachePreference, ""),
		strings.TrimSpace(firstNonEmpty(queryValue(query, func(q *GenerationQueueQuery) string { return q.Platform }), "")),
		strings.TrimSpace(firstNonEmpty(queryValue(query, func(q *GenerationQueueQuery) string { return q.Slot }), "")),
		strings.TrimSpace(firstNonEmpty(queryValue(query, func(q *GenerationQueueQuery) string { return q.PreviewCapability }), "")),
		strings.TrimSpace(firstNonEmpty(queryValue(query, func(q *GenerationQueueQuery) string { return q.AssetID }), "")),
		strings.TrimSpace(firstNonEmpty(queryValue(query, func(q *GenerationQueueQuery) string { return q.AssetRevision }), "")),
		strings.TrimSpace(firstNonEmpty(queryValue(query, func(q *GenerationQueueQuery) string { return q.PreviewRevision }), "")),
		strings.TrimSpace(firstNonEmpty(queryValue(query, func(q *GenerationQueueQuery) string { return q.TaskRevision }), "")),
	)
}

func generationNavigationDispatchExecutionPendingStep(step GenerationNavigationDispatchStep, dedupeKey string, responseMode string) *GenerationNavigationDispatchExecutionStep {
	return &GenerationNavigationDispatchExecutionStep{
		Kind:               step.Kind,
		ResponseMode:       firstNonEmpty(step.ResponseMode, responseMode),
		CachePreference:    step.CachePreference,
		RequiresRevalidate: step.RequiresRevalidate,
		DeduplicationKey:   dedupeKey,
		Status:             "pending",
	}
}

func generationNavigationDispatchPlanDeduplicatedStep(step GenerationNavigationDispatchStep, dedupeKey string, sourceIndex int) GenerationNavigationDispatchExecutionStep {
	return GenerationNavigationDispatchExecutionStep{
		Kind:               step.Kind,
		ResponseMode:       step.ResponseMode,
		CachePreference:    step.CachePreference,
		RequiresRevalidate: step.RequiresRevalidate,
		Status:             "deduplicated",
		DeduplicationKey:   dedupeKey,
		DeduplicatedFrom:   sourceIndex,
		Skipped:            true,
	}
}

func generationNavigationDispatchPlanSkippedStep(step GenerationNavigationDispatchStep, stopReason string) GenerationNavigationDispatchExecutionStep {
	return GenerationNavigationDispatchExecutionStep{
		Kind:               step.Kind,
		ResponseMode:       step.ResponseMode,
		CachePreference:    step.CachePreference,
		RequiresRevalidate: step.RequiresRevalidate,
		Status:             "skipped",
		Error:              stopReason,
		Skipped:            true,
	}
}

func applyGenerationNavigationDispatchExecutionStats(execution *GenerationNavigationDispatchExecution, step *GenerationNavigationDispatchExecutionStep) {
	if execution == nil || step == nil {
		return
	}
	switch step.Status {
	case "failed":
		execution.FailedSteps++
		execution.Partial = true
	case "deduplicated":
		execution.DedupedSteps++
	case "skipped":
		execution.Partial = true
	default:
		if step.Executed {
			execution.CompletedSteps++
		}
	}
}

func queryValue(query *GenerationQueueQuery, selector func(*GenerationQueueQuery) string) string {
	if query == nil || selector == nil {
		return ""
	}
	return selector(query)
}

func applyExecutedPlanToDispatchResponse(response *GenerationReviewNavigationDispatchResponse, execution *GenerationNavigationDispatchExecution) {
	applyGenerationNavigationDispatchExecutionMerge(response, execution)
}

func normalizeGenerationReviewDispatchKind(target *GenerationReviewNavigationTarget) string {
	if target == nil {
		return "session"
	}
	kind := strings.ToLower(strings.TrimSpace(target.DispatchKind))
	if kind != "" {
		return kind
	}
	if target.ActionTarget != nil {
		return "action"
	}
	if target.PreviewQuery != nil && strings.TrimSpace(target.PreviewQuery.AssetID) != "" {
		return "preview"
	}
	if target.SessionQuery != nil {
		return "session"
	}
	if target.QueueQuery != nil {
		return "queue"
	}
	return "session"
}

func normalizeGenerationNavigationDispatchPlanMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "execute_plan":
		return "execute_plan"
	default:
		return "primary_only"
	}
}

func cloneGenerationReviewNavigationTarget(target *GenerationReviewNavigationTarget) *GenerationReviewNavigationTarget {
	if target == nil {
		return nil
	}
	cloned := *target
	buildGenerationReviewNavigationTargetCloneShapePhase().run(target, &cloned)
	return applyIdentityToNavigationTarget(&cloned)
}

func finalizeGenerationReviewNavigationDispatchResponse(response *GenerationReviewNavigationDispatchResponse) *GenerationReviewNavigationDispatchResponse {
	if response == nil {
		return nil
	}
	response.PanelUpdate = buildGenerationReviewPanelUpdateFromDispatch(response)
	if (response.ReviewPreview != nil && response.ReviewPreview.NotModified) ||
		(response.Queue != nil && response.Queue.NotModified) ||
		(response.PanelUpdate != nil && response.PanelUpdate.NoChanges) {
		response.NotModified = true
	}
	return applyGenerationConditionalStateToNavigationDispatchResponse(response)
}
