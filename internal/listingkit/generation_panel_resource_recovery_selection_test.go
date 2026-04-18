package listingkit

import "testing"

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
