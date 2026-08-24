package sourceaccount

import "testing"

func TestSelectAccessModeUsesPublicForMissingAccount(t *testing.T) {
	mode, err := SelectAccessMode(0)
	if err != nil {
		t.Fatalf("SelectAccessMode(0) error = %v", err)
	}
	if mode != AccessModePublic {
		t.Fatalf("mode = %q, want %q", mode, AccessModePublic)
	}
}

func TestSelectAccessModeUsesAccountAssistedForPositiveAccount(t *testing.T) {
	mode, err := SelectAccessMode(42)
	if err != nil {
		t.Fatalf("SelectAccessMode(42) error = %v", err)
	}
	if mode != AccessModeAccountAssisted {
		t.Fatalf("mode = %q, want %q", mode, AccessModeAccountAssisted)
	}
}

func TestSelectAccessModeRejectsNegativeAccount(t *testing.T) {
	_, err := SelectAccessMode(-1)
	if err == nil {
		t.Fatal("SelectAccessMode(-1) error = nil, want invalid account error")
	}
	if got := ErrorCode(err); got != SourceAccountUnavailable {
		t.Fatalf("ErrorCode() = %q, want %q", got, SourceAccountUnavailable)
	}
}
