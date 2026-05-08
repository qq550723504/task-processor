package generation

func ReviewFocusKey(platform, slot, capability string) string {
	out := platform
	if slot != "" {
		if out != "" {
			out += ":"
		}
		out += slot
	}
	if capability != "" {
		if out != "" {
			out += ":"
		}
		out += capability
	}
	return out
}

func ActionInteractionMode(actionKey string) string {
	if PreviewCapabilityActionSpecForKey(actionKey) != nil {
		return "review_only"
	}
	switch actionKey {
	case ActionGenerateMissingAssets, ActionRetryFailedGeneration, ActionUpgradeFallbackAssets, ActionRetryProvisionalSlots:
		return "retryable"
	case ActionRetrySectionGeneration:
		return "retryable"
	case ActionReviewMissingSlots, ActionInspectFailedTasks:
		return "queue_only"
	case ActionReviewReadyAssets, ActionContinuePublishReview, ActionDeferSectionReview, ActionApproveSectionReview:
		return "review_only"
	default:
		return "queue_only"
	}
}
