package submission

import (
	"testing"
	"time"
)

func TestEnsureEventIDUsesExistingID(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 14, 15, 4, 5, 123, time.UTC)
	if got := EnsureEventID("custom-id", "submit_phase", now); got != "custom-id" {
		t.Fatalf("EnsureEventID() = %q, want existing id", got)
	}
}

func TestEnsureEventIDBuildsDefaultID(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 14, 15, 4, 5, 123, time.UTC)
	got := EnsureEventID("", "submit_phase", now)
	want := "submit_phase-1781449445000000123"
	if got != want {
		t.Fatalf("EnsureEventID() = %q, want %q", got, want)
	}
}

func TestPrependRecentEventsPrependsNewestFirst(t *testing.T) {
	t.Parallel()

	got := PrependRecentEvents([]string{"older", "oldest"}, "newest", 30)
	want := []string{"newest", "older", "oldest"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPrependRecentEventsTrimsToLimit(t *testing.T) {
	t.Parallel()

	events := []int{2, 1}
	got := PrependRecentEvents(events, 3, 2)
	want := []int{3, 2}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}
