package listingkit

import (
	"sort"
	"strings"
)

func cloneGenerationRecoverySummary(summary *GenerationRecoverySummary) *GenerationRecoverySummary {
	if summary == nil {
		return nil
	}
	cloned := *summary
	cloned.PrimaryDescriptor = cloneGenerationPanelResourceDescriptor(summary.PrimaryDescriptor)
	cloned.RecommendedDescriptors = cloneGenerationPanelResourceDescriptors(summary.RecommendedDescriptors)
	return &cloned
}

func cloneGenerationPanelResourceDescriptor(item *GenerationPanelResourceDescriptor) *GenerationPanelResourceDescriptor {
	if item == nil {
		return nil
	}
	cloned := *item
	cloned.Descriptor = cloneGenerationNavigationDescriptor(item.Descriptor)
	cloned.RecoveryTarget = cloneGenerationReviewNavigationTarget(item.RecoveryTarget)
	cloned.RecoveryDispatchPlan = cloneGenerationNavigationDispatchPlan(item.RecoveryDispatchPlan)
	return &cloned
}

func cloneGenerationPanelResourceDescriptors(items []GenerationPanelResourceDescriptor) []GenerationPanelResourceDescriptor {
	if len(items) == 0 {
		return nil
	}
	out := make([]GenerationPanelResourceDescriptor, 0, len(items))
	for _, item := range items {
		cloned := cloneGenerationPanelResourceDescriptor(&item)
		if cloned == nil {
			continue
		}
		out = append(out, *cloned)
	}
	return out
}

func selectGenerationPanelRecoveryDescriptors(items []GenerationPanelResourceDescriptor) (*GenerationPanelResourceDescriptor, []GenerationPanelResourceDescriptor) {
	if len(items) == 0 {
		return nil, nil
	}
	recoverable := make([]GenerationPanelResourceDescriptor, 0, len(items))
	for _, item := range items {
		if !isGenerationPanelResourceRecoverable(item) {
			continue
		}
		recoverable = append(recoverable, item)
	}
	if len(recoverable) == 0 {
		return nil, nil
	}
	sort.SliceStable(recoverable, func(i, j int) bool {
		li := generationRecoveryProfileForHint(recoverable[i].RecoveryHint).Priority
		lj := generationRecoveryProfileForHint(recoverable[j].RecoveryHint).Priority
		if li != lj {
			return li < lj
		}
		if recoverable[i].Role != recoverable[j].Role {
			return recoverable[i].Role < recoverable[j].Role
		}
		leftKey := ""
		if recoverable[i].Descriptor != nil {
			leftKey = recoverable[i].Descriptor.CacheKey
		}
		rightKey := ""
		if recoverable[j].Descriptor != nil {
			rightKey = recoverable[j].Descriptor.CacheKey
		}
		return leftKey < rightKey
	})
	primary := recoverable[0]
	return &primary, recoverable
}

func isGenerationPanelResourceRecoverable(item GenerationPanelResourceDescriptor) bool {
	if item.RecoveryHint == "" {
		return false
	}
	return item.RecoveryTarget != nil || item.Retryable
}

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

func applyGenerationPanelResourceRecoveryPresentation(item *GenerationPanelResourceDescriptor) {
	if item == nil {
		return
	}
	profile := generationRecoveryProfileForHint(item.RecoveryHint)
	item.RecoverySeverity = profile.Severity
	item.RecoveryUrgency = profile.Urgency
	item.RecoveryCTAKind = profile.CTAKind
}

type generationRecoveryProfile struct {
	Hint       string
	Priority   int
	Severity   string
	Urgency    string
	CTAKind    string
	Title      string
	Summary    string
	TitleKey   string
	SummaryKey string
}

var generationRecoveryProfiles = map[string]generationRecoveryProfile{
	"review_fallback": {
		Hint:       "review_fallback",
		Priority:   0,
		Severity:   "medium",
		Urgency:    "now",
		CTAKind:    "review",
		Title:      "Review Fallback Path",
		Summary:    "A fallback result is available and should be reviewed before retrying generation.",
		TitleKey:   "generation.recovery.review_fallback.title",
		SummaryKey: "generation.recovery.review_fallback.summary",
	},
	"retry_dispatch": {
		Hint:       "retry_dispatch",
		Priority:   1,
		Severity:   "high",
		Urgency:    "now",
		CTAKind:    "retry",
		Title:      "Retry Generation Step",
		Summary:    "A recoverable generation step failed and can be retried now.",
		TitleKey:   "generation.recovery.retry_dispatch.title",
		SummaryKey: "generation.recovery.retry_dispatch.summary",
	},
	"refresh_revision": {
		Hint:       "refresh_revision",
		Priority:   2,
		Severity:   "medium",
		Urgency:    "soon",
		CTAKind:    "refresh",
		Title:      "Refresh Resource Revision",
		Summary:    "The current revision is stale and should be refreshed before continuing.",
		TitleKey:   "generation.recovery.refresh_revision.title",
		SummaryKey: "generation.recovery.refresh_revision.summary",
	},
	"wait_for_generation": {
		Hint:       "wait_for_generation",
		Priority:   3,
		Severity:   "low",
		Urgency:    "later",
		CTAKind:    "monitor",
		Title:      "Wait For Generation",
		Summary:    "The asset is not ready yet. Refresh the queue after generation progresses.",
		TitleKey:   "generation.recovery.wait_for_generation.title",
		SummaryKey: "generation.recovery.wait_for_generation.summary",
	},
}

var defaultGenerationRecoveryProfile = generationRecoveryProfile{
	Priority:   4,
	Title:      "Review Recovery Options",
	Summary:    "Review available recovery actions for the current resource set.",
	TitleKey:   "generation.recovery.default.title",
	SummaryKey: "generation.recovery.default.summary",
}

func generationRecoveryProfileForHint(hint string) generationRecoveryProfile {
	normalized := strings.TrimSpace(strings.ToLower(hint))
	if profile, ok := generationRecoveryProfiles[normalized]; ok {
		return profile
	}
	return defaultGenerationRecoveryProfile
}

func shouldPreferRecoveryAsPrimaryCTA(primaryActionKey string, summary *GenerationRecoverySummary) bool {
	if summary == nil || summary.PrimaryDescriptor == nil || summary.PrimaryDescriptor.RecoveryTarget == nil {
		return false
	}
	if strings.TrimSpace(summary.Urgency) != "now" {
		return false
	}
	switch strings.TrimSpace(primaryActionKey) {
	case "",
		assetGenerationActionReviewReadyAssets,
		assetGenerationActionContinuePublishReview,
		assetGenerationActionReviewDetailPreviews,
		assetGenerationActionReviewMeasurementPreviews,
		assetGenerationActionReviewBadgePreviews,
		assetGenerationActionReviewCopyPreviews,
		assetGenerationActionReviewSubjectPreviews:
		return true
	default:
		return false
	}
}
