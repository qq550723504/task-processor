package submission

import "testing"

func TestPhaseDetail(t *testing.T) {
	t.Parallel()

	labels := PhaseDetailLabels{
		Validate:        "validate detail",
		PrepareProduct:  "prepare detail",
		UploadImages:    "upload detail",
		PreValidate:     "pre-validate detail",
		SubmitRemote:    "publish detail",
		SaveDraftRemote: "draft detail",
		PersistResult:   "persist detail",
		ConfirmRemote:   "confirm detail",
	}

	tests := []struct {
		name   string
		action string
		phase  string
		want   string
	}{
		{name: "validate", action: "publish", phase: "validate", want: "validate detail"},
		{name: "publish remote", action: "publish", phase: "submit_remote", want: "publish detail"},
		{name: "save draft remote", action: "save_draft", phase: "submit_remote", want: "draft detail"},
		{name: "unknown phase", action: "publish", phase: "custom_phase", want: "custom_phase"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := PhaseDetail(tt.action, tt.phase, labels); got != tt.want {
				t.Fatalf("PhaseDetail(%q, %q) = %q, want %q", tt.action, tt.phase, got, tt.want)
			}
		})
	}
}
