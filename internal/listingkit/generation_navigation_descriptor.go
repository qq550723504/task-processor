package listingkit

func buildGenerationNavigationDescriptor(target *GenerationReviewNavigationTarget) *GenerationNavigationDescriptor {
	if target == nil {
		return nil
	}
	return &GenerationNavigationDescriptor{
		ResourceKind:                 target.ResourceKind,
		CacheKey:                     target.CacheKey,
		CachePolicy:                  target.CachePolicy,
		SupportsStaleWhileRevalidate: target.CachePolicy == "stale_while_revalidate",
		RevalidateAfterAction:        target.RevalidateAfterAction,
		RefreshScope:                 buildGenerationNavigationRefreshScope(target),
		Invalidates:                  buildGenerationNavigationInvalidates(target),
		DispatchPlan:                 buildGenerationNavigationDispatchPlan(target),
		FollowUpReads:                buildGenerationNavigationFollowUpReads(target),
		Conditional:                  cloneGenerationConditionalState(target.Conditional),
	}
}

func buildGenerationNavigationRefreshScope(target *GenerationReviewNavigationTarget) string {
	switch buildGenerationNavigationTargetResourceKind(target) {
	case "generation_action":
		return "mutation"
	case "review_preview":
		return "focused_read"
	case "generation_queue":
		return "collection_read"
	default:
		return "panel_read"
	}
}

func buildGenerationNavigationInvalidates(target *GenerationReviewNavigationTarget) []string {
	switch buildGenerationNavigationTargetResourceKind(target) {
	case "generation_action":
		return []string{"review_session", "review_preview", "generation_queue"}
	case "review_preview":
		return []string{"review_preview"}
	case "generation_queue":
		return []string{"generation_queue"}
	default:
		return []string{"review_session"}
	}
}

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
		StopOnFirstSuccess: buildGenerationNavigationDispatchStopOnFirstSuccess(target, len(reads)),
		StopOnError:        buildGenerationNavigationDispatchStopOnError(target, len(reads)),
		FallbackStrategy:   buildGenerationNavigationDispatchFallbackStrategy(target, len(reads)),
		MaxParallelism:     buildGenerationNavigationDispatchMaxParallelism(target, len(reads)),
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
	switch buildGenerationNavigationTargetResourceKind(target) {
	case "generation_action":
		return "mutation_then_refresh"
	case "generation_queue", "review_preview", "review_session":
		if readCount <= 1 {
			return "single_read"
		}
		return "fanout_read"
	default:
		if readCount <= 1 {
			return "single_read"
		}
		return "fanout_read"
	}
}

func buildGenerationNavigationDispatchStopOnNotModified(target *GenerationReviewNavigationTarget, readCount int) bool {
	return readCount <= 1 && buildGenerationNavigationTargetResourceKind(target) != "generation_action"
}

func buildGenerationNavigationDispatchStopOnFirstSuccess(target *GenerationReviewNavigationTarget, readCount int) bool {
	return readCount <= 1 && buildGenerationNavigationTargetResourceKind(target) != "generation_action"
}

func buildGenerationNavigationDispatchStopOnError(target *GenerationReviewNavigationTarget, readCount int) bool {
	return readCount <= 1
}

func buildGenerationNavigationDispatchFallbackStrategy(target *GenerationReviewNavigationTarget, readCount int) string {
	switch buildGenerationNavigationTargetResourceKind(target) {
	case "generation_action":
		return "prefer_action_then_refresh_results"
	case "review_preview", "review_session":
		if readCount > 1 {
			return "prefer_preview_then_session_then_queue"
		}
		return "prefer_primary_only"
	case "generation_queue":
		return "prefer_queue_then_session"
	default:
		return "prefer_primary_only"
	}
}

func buildGenerationNavigationDispatchMaxParallelism(target *GenerationReviewNavigationTarget, readCount int) int {
	switch buildGenerationNavigationDispatchStrategy(target, readCount) {
	case "mutation_then_refresh":
		return 2
	case "fanout_read":
		return 3
	default:
		return 1
	}
}

func buildGenerationNavigationDispatchStepCachePreference(target *GenerationReviewNavigationTarget, stepKind string) string {
	if generationNavigationTargetRevalidateAfterAction(target) {
		return "revalidate"
	}
	switch stepKind {
	case "session", "preview":
		return "stale_while_revalidate"
	default:
		return "revalidate"
	}
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
