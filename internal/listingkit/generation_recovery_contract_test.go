package listingkit

import "testing"

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
