package publishing

import "testing"

func TestNormalizeFinalDraftSubmitModeAcceptsOnlySubmitActions(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		" PUBLISH ":    "publish",
		" save_draft ": "save_draft",
		"delete":       "",
		"":             "",
	}
	for input, want := range tests {
		if got := NormalizeFinalDraftSubmitMode(input); got != want {
			t.Fatalf("NormalizeFinalDraftSubmitMode(%q) = %q, want %q", input, got, want)
		}
	}
}
