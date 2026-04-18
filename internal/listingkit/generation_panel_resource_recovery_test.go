package listingkit

import "testing"

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
