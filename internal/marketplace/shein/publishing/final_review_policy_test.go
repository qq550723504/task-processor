package publishing

import "testing"

func TestFinalReviewMessageUsesActionSpecificCopy(t *testing.T) {
	t.Parallel()

	draft := FinalReviewMessage(" save_draft ")
	publish := FinalReviewMessage("publish")

	if draft == "" || publish == "" {
		t.Fatalf("messages should not be empty: draft=%q publish=%q", draft, publish)
	}
	if draft == publish {
		t.Fatalf("FinalReviewMessage(save_draft) = %q, want draft-specific copy", draft)
	}
	if got := FinalReviewMessage("unknown"); got != publish {
		t.Fatalf("FinalReviewMessage(unknown) = %q, want publish default %q", got, publish)
	}
}

func TestFinalReviewRequiredSkipsSaveDraftOnly(t *testing.T) {
	t.Parallel()

	if FinalReviewRequired(" save_draft ") {
		t.Fatal("FinalReviewRequired(save_draft) = true, want false")
	}
	for _, action := range []string{"publish", "", "unknown"} {
		if !FinalReviewRequired(action) {
			t.Fatalf("FinalReviewRequired(%q) = false, want true", action)
		}
	}
}

func TestSubmitActionAllowsReadinessBlockersForSaveDraftOnly(t *testing.T) {
	t.Parallel()

	if !SubmitActionAllowsReadinessBlockers(" save_draft ") {
		t.Fatal("SubmitActionAllowsReadinessBlockers(save_draft) = false, want true")
	}
	for _, action := range []string{"publish", "", "unknown"} {
		if SubmitActionAllowsReadinessBlockers(action) {
			t.Fatalf("SubmitActionAllowsReadinessBlockers(%q) = true, want false", action)
		}
	}
}
