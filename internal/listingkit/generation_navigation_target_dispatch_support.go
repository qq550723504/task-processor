package listingkit

import listinggeneration "task-processor/internal/listingkit/generation"

func buildGenerationNavigationFollowUpReads(target *GenerationReviewNavigationTarget) []GenerationNavigationFollowUpRead {
	if target == nil {
		return nil
	}
	baseQuery := generationNavigationDispatchBaseQuery(target)
	out := make([]GenerationNavigationFollowUpRead, 0, 3)
	seen := map[string]struct{}{}
	appendRead := func(kind string, query *GenerationQueueQuery, defaultResponseMode string) {
		if query == nil {
			return
		}
		if _, ok := seen[kind]; ok {
			return
		}
		responseMode := normalizeGenerationActionResponseMode(query.ResponseMode)
		if responseMode == "" {
			responseMode = normalizeGenerationActionResponseMode(defaultResponseMode)
		}
		out = append(out, GenerationNavigationFollowUpRead{
			Kind:         kind,
			ResponseMode: responseMode,
			Query:        cloneGenerationQueueQuery(query),
		})
		seen[kind] = struct{}{}
	}
	switch buildGenerationNavigationTargetResourceKind(target) {
	case "generation_action":
		appendRead("queue", firstNonNilQueueQuery(target.QueueQuery, baseQuery), "full")
		appendRead("session", firstNonNilQueueQuery(target.SessionQuery, baseQuery), "patch_only")
		appendRead("preview", firstNonNilQueueQuery(target.PreviewQuery, baseQuery), "full")
	case "review_preview", "review_session":
		appendRead("queue", firstNonNilQueueQuery(target.QueueQuery, baseQuery), "full")
		appendRead("session", firstNonNilQueueQuery(target.SessionQuery, baseQuery), "patch_only")
		appendRead("preview", firstNonNilQueueQuery(target.PreviewQuery, baseQuery), "full")
	default:
		appendRead("queue", firstNonNilQueueQuery(target.QueueQuery, baseQuery), "full")
	}
	return out
}

func buildGenerationNavigationDispatchPlan(target *GenerationReviewNavigationTarget) *GenerationNavigationDispatchPlan {
	reads := buildGenerationNavigationFollowUpReads(target)
	if len(reads) == 0 {
		return nil
	}
	plan := &GenerationNavigationDispatchPlan{
		Strategy:           buildGenerationNavigationDispatchStrategy(target, len(reads)),
		StopOnNotModified:  buildGenerationNavigationDispatchStopOnNotModified(target, len(reads)),
		StopOnFirstSuccess: listinggeneration.NavigationDispatchStopOnFirstSuccess(buildGenerationNavigationTargetResourceKind(target), len(reads)),
		StopOnError:        listinggeneration.NavigationDispatchStopOnError(len(reads)),
		FallbackStrategy:   listinggeneration.NavigationDispatchFallbackStrategy(buildGenerationNavigationTargetResourceKind(target), len(reads)),
		MaxParallelism:     listinggeneration.NavigationDispatchMaxParallelism(buildGenerationNavigationDispatchStrategy(target, len(reads))),
		DedupePolicy:       "by_step_identity",
		WinnerPolicy:       "prefer_preview_then_session_then_queue",
		RequiresRevalidate: generationNavigationTargetRevalidateAfterAction(target),
		Steps:              make([]GenerationNavigationDispatchStep, 0, len(reads)),
	}
	for _, item := range reads {
		cachePreference := buildGenerationNavigationDispatchStepCachePreference(target, item.Kind)
		plan.Steps = append(plan.Steps, GenerationNavigationDispatchStep{
			Kind:               item.Kind,
			ResponseMode:       item.ResponseMode,
			CachePreference:    cachePreference,
			RequiresRevalidate: cachePreference == "revalidate" || plan.RequiresRevalidate,
			Query:              cloneGenerationQueueQuery(item.Query),
		})
	}
	return plan
}

func buildGenerationNavigationDispatchStrategy(target *GenerationReviewNavigationTarget, readCount int) string {
	return listinggeneration.NavigationDispatchStrategy(buildGenerationNavigationTargetResourceKind(target), readCount)
}

func buildGenerationNavigationDispatchStopOnNotModified(target *GenerationReviewNavigationTarget, readCount int) bool {
	return listinggeneration.NavigationDispatchStopOnNotModified(buildGenerationNavigationTargetResourceKind(target), readCount)
}

func buildGenerationNavigationDispatchStepCachePreference(target *GenerationReviewNavigationTarget, stepKind string) string {
	return listinggeneration.NavigationDispatchStepCachePreference(generationNavigationTargetRevalidateAfterAction(target), stepKind)
}

func generationNavigationDispatchBaseQuery(target *GenerationReviewNavigationTarget) *GenerationQueueQuery {
	if target == nil {
		return nil
	}
	if target.ActionTarget != nil && target.ActionTarget.QueueQuery != nil {
		return cloneGenerationQueueQuery(target.ActionTarget.QueueQuery)
	}
	return cloneGenerationQueueQuery(primaryGenerationNavigationTargetQuery(target))
}

func firstNonNilQueueQuery(queries ...*GenerationQueueQuery) *GenerationQueueQuery {
	for _, query := range queries {
		if query != nil {
			return query
		}
	}
	return nil
}
