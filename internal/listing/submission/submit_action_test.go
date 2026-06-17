package submission

import "testing"

func TestNormalizeSubmitActionUsesExplicitValue(t *testing.T) {
	t.Parallel()

	if got := NormalizeSubmitAction(" SAVE_DRAFT ", "publish"); got != "save_draft" {
		t.Fatalf("NormalizeSubmitAction() = %q, want save_draft", got)
	}
}

func TestNormalizeSubmitActionFallsBackWhenEmpty(t *testing.T) {
	t.Parallel()

	if got := NormalizeSubmitAction("", " Publish "); got != "publish" {
		t.Fatalf("NormalizeSubmitAction() = %q, want publish", got)
	}
}

func TestIsSupportedSubmitAction(t *testing.T) {
	t.Parallel()

	for _, action := range []string{"publish", " save_draft ", "PUBLISH"} {
		if !IsSupportedSubmitAction(action) {
			t.Fatalf("IsSupportedSubmitAction(%q) = false, want true", action)
		}
	}
	if IsSupportedSubmitAction("delete") {
		t.Fatal("IsSupportedSubmitAction(delete) = true, want false")
	}
}

func TestPreferredSubmitActionUsesFirstSupportedCandidate(t *testing.T) {
	t.Parallel()

	if got := PreferredSubmitAction(" delete ", " SAVE_DRAFT ", "publish"); got != "save_draft" {
		t.Fatalf("PreferredSubmitAction() = %q, want save_draft", got)
	}
}

func TestPreferredSubmitActionReturnsEmptyWhenNoCandidateSupported(t *testing.T) {
	t.Parallel()

	if got := PreferredSubmitAction("delete", ""); got != "" {
		t.Fatalf("PreferredSubmitAction() = %q, want empty", got)
	}
}

func TestUnsupportedSubmitActionError(t *testing.T) {
	t.Parallel()

	if got := UnsupportedSubmitActionError("delete"); got == nil || got.Error() != "unsupported submit action: delete" {
		t.Fatalf("UnsupportedSubmitActionError() = %v, want unsupported submit action error", got)
	}
}
