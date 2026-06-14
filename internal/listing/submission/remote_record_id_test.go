package submission

import "testing"

func TestResolveRemoteRecordIDKeepsExplicitValue(t *testing.T) {
	t.Parallel()

	if got := ResolveRemoteRecordID("event-1", "record-1"); got != "event-1" {
		t.Fatalf("ResolveRemoteRecordID() = %q, want explicit event value", got)
	}
}

func TestResolveRemoteRecordIDFallsBackToRecordValue(t *testing.T) {
	t.Parallel()

	if got := ResolveRemoteRecordID("", "record-1"); got != "record-1" {
		t.Fatalf("ResolveRemoteRecordID() = %q, want record-1", got)
	}
}
