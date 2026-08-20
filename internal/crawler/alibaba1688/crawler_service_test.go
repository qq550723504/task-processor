package alibaba1688

import "testing"

func TestNewSourceAccessCountsInitializesAllAccessModes(t *testing.T) {
	counts := newSourceAccessCounts()

	for _, key := range []string{
		"public",
		"account_assisted",
		"source_public_unavailable",
		"source_account_unavailable",
		"source_account_disabled",
	} {
		value, ok := counts[key]
		if !ok || value != 0 {
			t.Fatalf("source access counter %q = %d/%t, want 0/true", key, value, ok)
		}
	}
}
