package listingkit

import "testing"

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
