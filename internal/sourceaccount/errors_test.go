package sourceaccount

import "testing"

func TestErrorCodeHidesAccountReferenceDetails(t *testing.T) {
	err := NewUnavailableError("tenant=101 account=42 secret-proxy")
	if got := ErrorCode(err); got != SourceAccountUnavailable {
		t.Fatalf("ErrorCode() = %q, want %q", got, SourceAccountUnavailable)
	}
	if err.Error() != "source account is unavailable" {
		t.Fatalf("error = %q, want sanitized message", err.Error())
	}
}
