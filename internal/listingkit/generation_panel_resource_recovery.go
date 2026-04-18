package listingkit

func applyGenerationPanelResourceRecovery(item *GenerationPanelResourceDescriptor) {
	if item == nil || item.RecoveryHint == "" {
		return
	}
	target, actionKey := buildGenerationPanelResourceRecoveryTarget(item)
	item.RecoveryActionKey = actionKey
	item.RecoveryTarget = target
	if target != nil && target.Descriptor != nil {
		item.RecoveryDispatchPlan = cloneGenerationNavigationDispatchPlan(target.Descriptor.DispatchPlan)
	}
	applyGenerationPanelResourceRecoveryPresentation(item)
}

func buildGenerationPanelResourceRecoveryTarget(item *GenerationPanelResourceDescriptor) (*GenerationReviewNavigationTarget, string) {
	if item == nil {
		return nil, ""
	}
	switch item.RecoveryHint {
	case "review_fallback":
		target := buildGenerationReviewNavigationTarget(item.Platform, item.Slot, item.Capability, nil)
		return target, reviewActionKeyForCapability(item.Capability)
	case "retry_dispatch":
		actionKey := assetGenerationActionRetrySectionGeneration
		actionTarget := &AssetGenerationActionTarget{
			ActionKey:       actionKey,
			InteractionMode: "review_only",
			QueueQuery: &GenerationQueueQuery{
				Platform:          item.Platform,
				Slot:              item.Slot,
				PreviewCapability: item.Capability,
			},
		}
		return buildGenerationReviewActionNavigationTarget(actionTarget), actionKey
	case "refresh_revision", "wait_for_generation":
		return applyIdentityToNavigationTarget(&GenerationReviewNavigationTarget{
			DispatchKind: "queue",
			QueueQuery: &GenerationQueueQuery{
				Platform:          item.Platform,
				Slot:              item.Slot,
				PreviewCapability: item.Capability,
			},
		}), ""
	default:
		return nil, ""
	}
}
