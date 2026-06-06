package listingkit

import "testing"

func TestBuildGenerationRecoverySummaryFromDescriptors(t *testing.T) {
	t.Parallel()

	descriptors := []GenerationPanelResourceDescriptor{
		{
			Role:              "queue_item",
			RecoveryHint:      "retry_dispatch",
			RecoverySeverity:  "high",
			RecoveryUrgency:   "now",
			RecoveryCTAKind:   "retry",
			RecoveryActionKey: assetGenerationActionRetrySectionGeneration,
			Retryable:         true,
			RecoveryTarget:    &GenerationReviewNavigationTarget{DispatchKind: "action"},
		},
		{
			Role:             "focused_preview",
			RecoveryHint:     "review_fallback",
			RecoverySeverity: "medium",
			RecoveryUrgency:  "now",
			RecoveryCTAKind:  "review",
			RecoveryTarget:   &GenerationReviewNavigationTarget{DispatchKind: "session"},
		},
	}

	summary := buildGenerationRecoverySummaryFromDescriptors(descriptors)
	if summary == nil {
		t.Fatalf("summary = nil, want recovery summary")
	}
	if summary.Title != "Review Fallback Path" || summary.CTAKind != "review" || summary.RecommendedCount != 2 {
		t.Fatalf("summary = %+v, want prioritized review fallback summary", summary)
	}
	if summary.PrimaryDescriptor == nil || summary.PrimaryDescriptor.RecoveryHint != "review_fallback" {
		t.Fatalf("primary descriptor = %+v, want review_fallback primary descriptor", summary.PrimaryDescriptor)
	}
}

func TestSelectGenerationPanelRecoveryDescriptorsPrefersReviewFallback(t *testing.T) {
	t.Parallel()

	items := []GenerationPanelResourceDescriptor{
		{
			Role:         "queue_item",
			RecoveryHint: "retry_dispatch",
			Retryable:    true,
			RecoveryTarget: &GenerationReviewNavigationTarget{
				DispatchKind: "action",
			},
		},
		{
			Role:         "focused_preview",
			RecoveryHint: "review_fallback",
			Retryable:    false,
			RecoveryTarget: &GenerationReviewNavigationTarget{
				DispatchKind: "session",
			},
		},
	}

	primary, recommended := selectGenerationPanelRecoveryDescriptors(items)

	if primary == nil || primary.RecoveryHint != "review_fallback" {
		t.Fatalf("primary = %+v, want review_fallback primary recovery", primary)
	}
	if len(recommended) != 2 || recommended[0].RecoveryHint != "review_fallback" || recommended[1].RecoveryHint != "retry_dispatch" {
		t.Fatalf("recommended = %+v, want ordered recovery descriptors", recommended)
	}
	if recommended[0].RecoveryCTAKind != "" || recommended[1].RecoveryCTAKind != "" {
		t.Fatalf("recommended = %+v, want selection helper to preserve descriptor shape only", recommended)
	}
}

func TestGenerationRecoveryProfileForHintProvidesCentralizedDefaults(t *testing.T) {
	t.Parallel()

	profile := generationRecoveryProfileForHint("review_fallback")
	if profile.Priority != 0 || profile.Severity != "medium" || profile.Urgency != "now" || profile.CTAKind != "review" {
		t.Fatalf("profile = %+v, want centralized review_fallback defaults", profile)
	}
	if profile.Title == "" || profile.Summary == "" || profile.TitleKey == "" || profile.SummaryKey == "" {
		t.Fatalf("profile = %+v, want populated summary metadata", profile)
	}

	fallback := generationRecoveryProfileForHint("unknown_hint")
	if fallback.Priority != 4 || fallback.Title == "" || fallback.Summary == "" {
		t.Fatalf("fallback profile = %+v, want default profile", fallback)
	}
}

func TestApplyGenerationPanelResourceRecoveryBuildsRetryDispatchTarget(t *testing.T) {
	t.Parallel()

	item := &GenerationPanelResourceDescriptor{
		Platform:      "shein",
		Slot:          "main",
		Capability:    "detail_preview",
		RecoveryScope: "queue_item",
		RecoveryHint:  "retry_dispatch",
		Retryable:     true,
	}

	applyGenerationPanelResourceRecovery(item)

	if item.RecoveryActionKey != assetGenerationActionRetrySectionGeneration {
		t.Fatalf("recovery action key = %q, want %q", item.RecoveryActionKey, assetGenerationActionRetrySectionGeneration)
	}
	if item.RecoveryTarget == nil || item.RecoveryTarget.DispatchKind != "action" || item.RecoveryTarget.ActionTarget == nil {
		t.Fatalf("recovery target = %+v, want action recovery target", item.RecoveryTarget)
	}
	if item.RecoveryDispatchPlan == nil || item.RecoveryDispatchPlan.Strategy != "mutation_then_refresh" {
		t.Fatalf("recovery dispatch plan = %+v, want mutation_then_refresh plan", item.RecoveryDispatchPlan)
	}
	if item.RecoverySeverity != "high" || item.RecoveryUrgency != "now" || item.RecoveryCTAKind != "retry" {
		t.Fatalf("recovery presentation = %+v, want retry presentation metadata", item)
	}
}

func TestApplyGenerationPanelResourceRecoveryBuildsReviewFallbackTarget(t *testing.T) {
	t.Parallel()

	item := &GenerationPanelResourceDescriptor{
		Platform:      "shein",
		Slot:          "main",
		Capability:    "detail_preview",
		RecoveryScope: "focused_resource",
		RecoveryHint:  "review_fallback",
		Retryable:     false,
	}

	applyGenerationPanelResourceRecovery(item)

	if item.RecoveryActionKey != assetGenerationActionReviewDetailPreviews {
		t.Fatalf("recovery action key = %q, want %q", item.RecoveryActionKey, assetGenerationActionReviewDetailPreviews)
	}
	if item.RecoveryTarget == nil || item.RecoveryTarget.DispatchKind != "session" {
		t.Fatalf("recovery target = %+v, want session review target", item.RecoveryTarget)
	}
	if item.RecoveryDispatchPlan == nil || item.RecoveryDispatchPlan.Strategy != "fanout_read" {
		t.Fatalf("recovery dispatch plan = %+v, want fanout review plan", item.RecoveryDispatchPlan)
	}
	if item.RecoverySeverity != "medium" || item.RecoveryUrgency != "now" || item.RecoveryCTAKind != "review" {
		t.Fatalf("recovery presentation = %+v, want review presentation metadata", item)
	}
}

func TestApplyGenerationRecoverySummaryToIndependentResponses(t *testing.T) {
	t.Parallel()

	descriptor := newRecoveryContractDescriptor()

	queue := applyGenerationRecoverySummaryToQueuePage(&GenerationQueuePage{
		ResourceDescriptors: []GenerationPanelResourceDescriptor{descriptor},
	})
	queue = applyGenerationResolvedActionSummaryToQueuePage(queue)
	if queue.RecoverySummary == nil || queue.RecoverySummary.ActionKey != assetGenerationActionReviewDetailPreviews {
		t.Fatalf("queue recovery summary = %+v, want review recovery summary", queue.RecoverySummary)
	}
	if queue.ResolvedActionSummary == nil || queue.ResolvedActionSummary.SourceKind != "recovery" {
		t.Fatalf("queue resolved action summary = %+v, want recovery resolved summary", queue.ResolvedActionSummary)
	}

	session := applyGenerationRecoverySummaryToReviewSessionResponse(&GenerationReviewSessionResponse{
		ResourceDescriptors: []GenerationPanelResourceDescriptor{descriptor},
		Session: &GenerationReviewSession{
			FocusedTarget: buildGenerationReviewTarget("shein", "main", "detail_preview"),
		},
	})
	session = applyGenerationResolvedActionSummaryToReviewSessionResponse(session)
	if session.RecoverySummary == nil || session.RecoverySummary.ActionKey != assetGenerationActionReviewDetailPreviews {
		t.Fatalf("session recovery summary = %+v, want review recovery summary", session.RecoverySummary)
	}
	if session.ResolvedActionSummary == nil || session.ResolvedActionSummary.ActionKey != assetGenerationActionReviewDetailPreviews {
		t.Fatalf("session resolved action summary = %+v, want detail review summary", session.ResolvedActionSummary)
	}

	preview := applyGenerationRecoverySummaryToReviewPreviewResponse(&GenerationReviewPreviewResponse{
		ResourceDescriptors: []GenerationPanelResourceDescriptor{descriptor},
		ReviewTarget:        buildGenerationReviewTarget("shein", "main", "detail_preview"),
	})
	preview = applyGenerationResolvedActionSummaryToReviewPreviewResponse(preview)
	if preview.RecoverySummary == nil || preview.RecoverySummary.ActionKey != assetGenerationActionReviewDetailPreviews {
		t.Fatalf("preview recovery summary = %+v, want review recovery summary", preview.RecoverySummary)
	}
	if preview.ResolvedActionSummary == nil || preview.ResolvedActionSummary.ActionKey != assetGenerationActionReviewDetailPreviews {
		t.Fatalf("preview resolved action summary = %+v, want detail review summary", preview.ResolvedActionSummary)
	}

	action := applyGenerationRecoverySummaryToActionResult(&GenerationActionExecutionResult{
		ResourceDescriptors: []GenerationPanelResourceDescriptor{descriptor},
		ResolvedTarget: &AssetGenerationActionTarget{
			ActionKey: assetGenerationActionRetrySectionGeneration,
		},
	})
	action = applyGenerationResolvedActionSummaryToActionResult(action)
	if action.RecoverySummary == nil || action.RecoverySummary.ActionKey != assetGenerationActionReviewDetailPreviews {
		t.Fatalf("action recovery summary = %+v, want review recovery summary", action.RecoverySummary)
	}
	if action.ResolvedActionSummary == nil || action.ResolvedActionSummary.ActionKey != assetGenerationActionRetrySectionGeneration {
		t.Fatalf("action resolved action summary = %+v, want retry section summary", action.ResolvedActionSummary)
	}
}

func TestBuildAssetGenerationOverviewArbitratesPrimaryRecoveryCTA(t *testing.T) {
	t.Parallel()

	queue := &GenerationWorkQueue{
		Summary: &GenerationWorkQueueSummary{
			QualityGradeCounts: map[string]int{"ideal": 1},
			PlatformQualityGradeCounts: map[string]map[string]int{
				"shein": {"ideal": 1},
			},
		},
		Items: []GenerationWorkQueueItem{{
			Platform:            "shein",
			Slot:                "main",
			PreviewCapabilities: []string{"detail_preview"},
			RetryHint:           "review_fallback",
		}},
	}

	overview := buildAssetGenerationOverview(queue)
	if overview == nil {
		t.Fatal("overview = nil")
	}
	if overview.PrimaryActionKey != assetGenerationActionContinuePublishReview {
		t.Fatalf("primary action key = %q, want continue_publish_review", overview.PrimaryActionKey)
	}
	if overview.PrimaryCTAKind != "recovery" {
		t.Fatalf("primary cta kind = %q, want recovery", overview.PrimaryCTAKind)
	}
	if overview.PrimaryNavigationTarget == nil || overview.PrimaryNavigationTarget.DispatchKind != "session" {
		t.Fatalf("primary navigation target = %+v, want recovery review target", overview.PrimaryNavigationTarget)
	}
	if overview.RecoverySummary == nil || overview.RecoverySummary.ActionKey != assetGenerationActionReviewDetailPreviews {
		t.Fatalf("recovery summary = %+v, want detail review recovery", overview.RecoverySummary)
	}
	if overview.ResolvedActionSummary == nil || overview.ResolvedActionSummary.SourceKind != "recovery" {
		t.Fatalf("resolved action summary = %+v, want recovery summary", overview.ResolvedActionSummary)
	}
	if overview.ResolvedActionSummary.NavigationTarget == nil || overview.ResolvedActionSummary.NavigationTarget.DispatchKind != "session" {
		t.Fatalf("resolved action navigation target = %+v, want session recovery target", overview.ResolvedActionSummary)
	}
}

func TestApplyGenerationRecoveryArbitrationToPlatformCard(t *testing.T) {
	t.Parallel()

	card := ListingKitPlatformCard{
		PrimaryActionKey: assetGenerationActionReviewReadyAssets,
		RecoverySummary:  buildGenerationRecoverySummaryFromDescriptors([]GenerationPanelResourceDescriptor{newRecoveryContractDescriptor()}),
	}

	applyGenerationRecoveryArbitrationToPlatformCard(&card)
	if card.PrimaryCTAKind != "recovery" {
		t.Fatalf("primary cta kind = %q, want recovery", card.PrimaryCTAKind)
	}
	if card.PrimaryNavigationTarget == nil || card.PrimaryNavigationTarget.DispatchKind != "session" {
		t.Fatalf("primary navigation target = %+v, want session recovery target", card.PrimaryNavigationTarget)
	}
	if card.ResolvedActionSummary == nil || card.ResolvedActionSummary.SourceKind != "recovery" {
		t.Fatalf("resolved action summary = %+v, want recovery resolved summary", card.ResolvedActionSummary)
	}
}

func TestBuildAssetGenerationOverviewBuildsGenerationActionResolvedSummary(t *testing.T) {
	t.Parallel()

	queue := &GenerationWorkQueue{
		Summary: &GenerationWorkQueueSummary{
			MissingItems:       1,
			RetryableItems:     1,
			QualityGradeCounts: map[string]int{"missing": 1},
			PlatformQualityGradeCounts: map[string]map[string]int{
				"amazon": {"missing": 1},
			},
		},
	}

	overview := buildAssetGenerationOverview(queue)
	if overview == nil || overview.ResolvedActionSummary == nil {
		t.Fatalf("overview = %+v, want resolved action summary", overview)
	}
	if overview.ResolvedActionSummary.SourceKind != "generation_action" {
		t.Fatalf("resolved action summary = %+v, want generation_action source", overview.ResolvedActionSummary)
	}
	if overview.ResolvedActionSummary.ActionTarget == nil || overview.ResolvedActionSummary.ActionTarget.ActionKey != assetGenerationActionGenerateMissingAssets {
		t.Fatalf("resolved action summary = %+v, want missing-assets action target", overview.ResolvedActionSummary)
	}
	if overview.ResolvedActionSummary.NavigationTarget == nil || overview.ResolvedActionSummary.NavigationTarget.DispatchKind != "action" {
		t.Fatalf("resolved action summary = %+v, want action navigation target", overview.ResolvedActionSummary)
	}
}

func TestApplyGenerationResolvedActionSummaryToNavigationDispatchResponsePrefersAction(t *testing.T) {
	t.Parallel()

	response := &GenerationReviewNavigationDispatchResponse{
		Action: &GenerationActionExecutionResult{
			ResolvedActionSummary: &GenerationResolvedActionSummary{
				SourceKind: "generation_action",
				ActionKey:  assetGenerationActionRetrySectionGeneration,
			},
		},
		ReviewSession: &GenerationReviewSessionResponse{
			ResolvedActionSummary: &GenerationResolvedActionSummary{
				SourceKind: "review_target",
				ActionKey:  assetGenerationActionReviewDetailPreviews,
			},
		},
	}

	response = applyGenerationResolvedActionSummaryToNavigationDispatchResponse(response)
	if response.ResolvedActionSummary == nil || response.ResolvedActionSummary.ActionKey != assetGenerationActionRetrySectionGeneration {
		t.Fatalf("dispatch resolved action summary = %+v, want action summary to win", response.ResolvedActionSummary)
	}
}

func newRecoveryContractDescriptor() GenerationPanelResourceDescriptor {
	item := GenerationPanelResourceDescriptor{
		Role:       "queue_item",
		Platform:   "shein",
		Slot:       "main",
		Capability: "detail_preview",
		Descriptor: &GenerationNavigationDescriptor{
			ResourceKind: "generation_queue",
			CacheKey:     "queue:shein:main:detail_preview",
		},
		RecoveryHint: "review_fallback",
	}
	applyGenerationPanelResourceRecovery(&item)
	return item
}
