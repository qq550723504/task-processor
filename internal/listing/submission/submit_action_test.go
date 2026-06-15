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
