package submission

import "testing"

func TestBuildRefreshRemotePolicyMarksPublishAcceptedAsDefaultConfirmed(t *testing.T) {
	t.Parallel()

	policy := BuildRefreshRemotePolicy("publish", true)
	if !policy.DefaultConfirmed {
		t.Fatal("DefaultConfirmed = false, want true")
	}
	if policy.FallbackMessage != "" {
		t.Fatalf("FallbackMessage = %q, want empty", policy.FallbackMessage)
	}
}

func TestBuildRefreshRemotePolicyDoesNotConfirmOtherActions(t *testing.T) {
	t.Parallel()

	policy := BuildRefreshRemotePolicy("save_draft", true)
	if policy.DefaultConfirmed {
		t.Fatal("DefaultConfirmed = true, want false for save_draft")
	}
}
