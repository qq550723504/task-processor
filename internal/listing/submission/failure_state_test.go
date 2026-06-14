package submission

import "testing"

func TestResolveFailureState(t *testing.T) {
	t.Parallel()

	requestID, phase := ResolveFailureState(" explicit ", " persist_result ", "current-id", "submit_remote", "validate")
	if requestID != "explicit" || phase != "persist_result" {
		t.Fatalf("ResolveFailureState(explicit) = %q/%q, want explicit/persist_result", requestID, phase)
	}

	requestID, phase = ResolveFailureState("", "", "current-id", "submit_remote", "validate")
	if requestID != "current-id" || phase != "submit_remote" {
		t.Fatalf("ResolveFailureState(current) = %q/%q, want current-id/submit_remote", requestID, phase)
	}

	requestID, phase = ResolveFailureState("", "", "", "", "validate")
	if requestID != "" || phase != "validate" {
		t.Fatalf("ResolveFailureState(default) = %q/%q, want empty/validate", requestID, phase)
	}
}
