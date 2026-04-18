package listingkit

import "strings"

func applyIdentityToNavigationTarget(target *GenerationReviewNavigationTarget) *GenerationReviewNavigationTarget {
	if target == nil {
		return nil
	}
	target.ResourceKind = buildGenerationNavigationTargetResourceKind(target)
	target.CacheKey = buildGenerationNavigationTargetCacheKey(target)
	target.CachePolicy = buildGenerationNavigationTargetCachePolicy(target)
	target.RevalidateAfterAction = generationNavigationTargetRevalidateAfterAction(target)
	target.Descriptor = buildGenerationNavigationDescriptor(target)
	return target
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
	switch buildGenerationNavigationTargetResourceKind(target) {
	case "generation_action":
		return "network_only"
	case "generation_queue":
		return "revalidate"
	case "review_preview", "review_session":
		return "stale_while_revalidate"
	default:
		return "revalidate"
	}
}

func generationNavigationTargetRevalidateAfterAction(target *GenerationReviewNavigationTarget) bool {
	if target == nil {
		return false
	}
	switch buildGenerationNavigationTargetResourceKind(target) {
	case "generation_action", "generation_queue", "review_session", "review_preview":
		if buildGenerationNavigationTargetResourceKind(target) == "generation_action" {
			return true
		}
		return false
	default:
		return false
	}
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
