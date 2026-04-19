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
