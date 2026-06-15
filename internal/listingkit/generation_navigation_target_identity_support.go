package listingkit

import (
	"strings"

	listinggeneration "task-processor/internal/listingkit/generation"
)

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
	return listinggeneration.NavigationRefreshScope(buildGenerationNavigationTargetResourceKind(target))
}

func buildGenerationNavigationInvalidates(target *GenerationReviewNavigationTarget) []string {
	return listinggeneration.NavigationInvalidates(buildGenerationNavigationTargetResourceKind(target))
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
	return listinggeneration.NavigationDispatchStrategy(buildGenerationNavigationTargetResourceKind(target), readCount)
}

func buildGenerationNavigationDispatchStopOnNotModified(target *GenerationReviewNavigationTarget, readCount int) bool {
	return listinggeneration.NavigationDispatchStopOnNotModified(buildGenerationNavigationTargetResourceKind(target), readCount)
}

func buildGenerationNavigationDispatchStopOnFirstSuccess(target *GenerationReviewNavigationTarget, readCount int) bool {
	return listinggeneration.NavigationDispatchStopOnFirstSuccess(buildGenerationNavigationTargetResourceKind(target), readCount)
}

func buildGenerationNavigationDispatchStopOnError(target *GenerationReviewNavigationTarget, readCount int) bool {
	return listinggeneration.NavigationDispatchStopOnError(readCount)
}

func buildGenerationNavigationDispatchFallbackStrategy(target *GenerationReviewNavigationTarget, readCount int) string {
	return listinggeneration.NavigationDispatchFallbackStrategy(buildGenerationNavigationTargetResourceKind(target), readCount)
}

func buildGenerationNavigationDispatchMaxParallelism(target *GenerationReviewNavigationTarget, readCount int) int {
	return listinggeneration.NavigationDispatchMaxParallelism(buildGenerationNavigationDispatchStrategy(target, readCount))
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

func buildGenerationNavigationTargetResourceKind(target *GenerationReviewNavigationTarget) string {
	if target == nil {
		return ""
	}
	switch normalizeGenerationReviewDispatchKind(target) {
	case "action":
		return "generation_action"
	case "preview":
		return "review_preview"
	case "queue":
		return "generation_queue"
	default:
		return "review_session"
	}
}

func buildGenerationNavigationTargetCacheKey(target *GenerationReviewNavigationTarget) string {
	if target == nil {
		return ""
	}
	kind := buildGenerationNavigationTargetResourceKind(target)
	query := primaryGenerationNavigationTargetQuery(target)
	actionKey := ""
	if target.ActionTarget != nil {
		actionKey = strings.TrimSpace(target.ActionTarget.ActionKey)
	}
	if query == nil {
		return hashRenderRevision(kind, actionKey)
	}
	return hashRenderRevision(
		kind,
		strings.TrimSpace(query.Platform),
		strings.TrimSpace(query.Slot),
		strings.TrimSpace(query.PreviewCapability),
		strings.TrimSpace(query.AssetID),
		strings.TrimSpace(query.AssetRevision),
		strings.TrimSpace(query.PreviewRevision),
		strings.TrimSpace(query.TaskRevision),
		actionKey,
	)
}

func buildGenerationNavigationTargetCachePolicy(target *GenerationReviewNavigationTarget) string {
	if target == nil {
		return ""
	}
	return listinggeneration.NavigationCachePolicy(buildGenerationNavigationTargetResourceKind(target))
}

func generationNavigationTargetRevalidateAfterAction(target *GenerationReviewNavigationTarget) bool {
	if target == nil {
		return false
	}
	return listinggeneration.NavigationRevalidateAfterAction(buildGenerationNavigationTargetResourceKind(target))
}

func primaryGenerationNavigationTargetQuery(target *GenerationReviewNavigationTarget) *GenerationQueueQuery {
	if target == nil {
		return nil
	}
	switch normalizeGenerationReviewDispatchKind(target) {
	case "preview":
		if target.PreviewQuery != nil {
			return target.PreviewQuery
		}
		if target.SessionQuery != nil {
			return target.SessionQuery
		}
		return target.QueueQuery
	case "queue":
		if target.QueueQuery != nil {
			return target.QueueQuery
		}
		if target.SessionQuery != nil {
			return target.SessionQuery
		}
		return target.PreviewQuery
	case "action":
		if target.ActionTarget != nil && target.ActionTarget.QueueQuery != nil {
			return target.ActionTarget.QueueQuery
		}
		return target.QueueQuery
	default:
		if target.SessionQuery != nil {
			return target.SessionQuery
		}
		if target.QueueQuery != nil {
			return target.QueueQuery
		}
		return target.PreviewQuery
	}
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
