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
