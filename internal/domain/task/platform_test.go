package task

import "testing"

func TestNormalizePlatformCanonicalizesCaseAndWhitespace(t *testing.T) {
	if got := NormalizePlatform("  SHEIN "); got != "shein" {
		t.Fatalf("NormalizePlatform() = %q, want shein", got)
	}
}
