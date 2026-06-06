package listingkit

import (
	"sort"
	"strings"
)

func buildGenerationRecoverySummaryFromQueue(queue *GenerationWorkQueue) *GenerationRecoverySummary {
	if queue == nil {
		return nil
	}
	descriptors := buildGenerationQueueResponseDescriptors(&GenerationQueuePage{Items: queue.Items})
	return buildGenerationRecoverySummaryFromDescriptors(descriptors)
}

func buildGenerationRecoverySummaryFromDescriptors(items []GenerationPanelResourceDescriptor) *GenerationRecoverySummary {
	primary, recommended := buildGenerationPanelRecoverySelections(items)
	if primary == nil {
		return nil
	}
	profile := generationRecoveryProfileForHint(primary.RecoveryHint)
	return &GenerationRecoverySummary{
		Title:                  profile.Title,
		Summary:                profile.Summary,
		Severity:               primary.RecoverySeverity,
		Urgency:                primary.RecoveryUrgency,
		CTAKind:                primary.RecoveryCTAKind,
		ActionKey:              primary.RecoveryActionKey,
		RecommendedCount:       len(recommended),
		PrimaryDescriptor:      cloneGenerationPanelResourceDescriptor(primary),
		RecommendedDescriptors: cloneGenerationPanelResourceDescriptors(recommended),
	}
}

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

func buildGenerationPanelRecoverySelections(items []GenerationPanelResourceDescriptor) (*GenerationPanelResourceDescriptor, []GenerationPanelResourceDescriptor) {
	primary, recommended := selectGenerationPanelRecoveryDescriptors(items)
	return primary, recommended
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
		li := generationPanelRecoveryPriority(recoverable[i])
		lj := generationPanelRecoveryPriority(recoverable[j])
		if li != lj {
			return li < lj
		}
		if recoverable[i].Role != recoverable[j].Role {
			return recoverable[i].Role < recoverable[j].Role
		}
		return generationPanelResourceCacheKey(recoverable[i]) < generationPanelResourceCacheKey(recoverable[j])
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

func generationPanelRecoveryPriority(item GenerationPanelResourceDescriptor) int {
	return generationRecoveryProfileForHint(item.RecoveryHint).Priority
}

func generationPanelResourceCacheKey(item GenerationPanelResourceDescriptor) string {
	if item.Descriptor == nil {
		return ""
	}
	return item.Descriptor.CacheKey
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

func applyGenerationRecoverySummaryToQueuePage(page *GenerationQueuePage) *GenerationQueuePage {
	if page == nil {
		return nil
	}
	page.RecoverySummary = buildGenerationRecoverySummaryFromDescriptors(page.ResourceDescriptors)
	return page
}

func applyGenerationRecoverySummaryToReviewSessionResponse(response *GenerationReviewSessionResponse) *GenerationReviewSessionResponse {
	if response == nil {
		return nil
	}
	response.RecoverySummary = buildGenerationRecoverySummaryFromDescriptors(response.ResourceDescriptors)
	return response
}

func applyGenerationRecoverySummaryToReviewPreviewResponse(response *GenerationReviewPreviewResponse) *GenerationReviewPreviewResponse {
	if response == nil {
		return nil
	}
	response.RecoverySummary = buildGenerationRecoverySummaryFromDescriptors(response.ResourceDescriptors)
	return response
}

func applyGenerationRecoverySummaryToActionResult(result *GenerationActionExecutionResult) *GenerationActionExecutionResult {
	if result == nil {
		return nil
	}
	result.RecoverySummary = buildGenerationRecoverySummaryFromDescriptors(result.ResourceDescriptors)
	return result
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

func applyGenerationRecoveryArbitrationToOverview(overview *AssetGenerationOverview) *AssetGenerationOverview {
	if overview == nil {
		return nil
	}
	overview.RecoverySummary = cloneGenerationRecoverySummary(overview.RecoverySummary)
	overview.PrimaryCTAKind = "generation_action"
	if overview.PrimaryActionTarget != nil {
		overview.PrimaryNavigationTarget = cloneGenerationReviewNavigationTarget(overview.PrimaryActionTarget.NavigationTarget)
	}
	if !shouldPreferRecoveryAsPrimaryCTA(overview.PrimaryActionKey, overview.RecoverySummary) {
		return overview
	}
	overview.PrimaryCTAKind = "recovery"
	overview.PrimaryNavigationTarget = cloneGenerationReviewNavigationTarget(overview.RecoverySummary.PrimaryDescriptor.RecoveryTarget)
	if strings.TrimSpace(overview.PrimaryActionReason) == "" {
		overview.PrimaryActionReason = overview.RecoverySummary.Summary
	} else if !strings.Contains(overview.PrimaryActionReason, overview.RecoverySummary.Summary) {
		overview.PrimaryActionReason = overview.RecoverySummary.Summary + " " + overview.PrimaryActionReason
	}
	overview.ResolvedActionSummary = buildGenerationResolvedActionSummaryFromOverview(overview)
	return overview
}

func finalizeGenerationOverviewActionSummary(overview *AssetGenerationOverview) *AssetGenerationOverview {
	if overview == nil {
		return nil
	}
	overview.ResolvedActionSummary = buildGenerationResolvedActionSummaryFromOverview(overview)
	return overview
}

func applyGenerationRecoveryArbitrationToPlatformCard(card *ListingKitPlatformCard) {
	if card == nil {
		return
	}
	card.RecoverySummary = cloneGenerationRecoverySummary(card.RecoverySummary)
	card.PrimaryCTAKind = "generation_action"
	if card.PrimaryActionTarget != nil {
		card.PrimaryNavigationTarget = cloneGenerationReviewNavigationTarget(card.PrimaryActionTarget.NavigationTarget)
	}
	if !shouldPreferRecoveryAsPrimaryCTA(card.PrimaryActionKey, card.RecoverySummary) {
		card.ResolvedActionSummary = buildGenerationResolvedActionSummaryFromPlatformCard(card)
		return
	}
	card.PrimaryCTAKind = "recovery"
	card.PrimaryNavigationTarget = cloneGenerationReviewNavigationTarget(card.RecoverySummary.PrimaryDescriptor.RecoveryTarget)
	card.ResolvedActionSummary = buildGenerationResolvedActionSummaryFromPlatformCard(card)
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
