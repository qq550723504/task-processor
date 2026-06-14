package submission

import "testing"

func TestResolveRefreshRequestIDTrimsValue(t *testing.T) {
	t.Parallel()

	if got := ResolveRefreshRequestID("  refresh-123  "); got != "refresh-123" {
		t.Fatalf("ResolveRefreshRequestID() = %q, want refresh-123", got)
	}
}

func TestResolveRefreshRequestIDEmptyWhenBlank(t *testing.T) {
	t.Parallel()

	if got := ResolveRefreshRequestID("   "); got != "" {
		t.Fatalf("ResolveRefreshRequestID() = %q, want empty string", got)
	}
}
