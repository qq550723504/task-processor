package submission

import "testing"

func TestRefreshActionMatchesFallsBackToRequestedWhenCurrentMissing(t *testing.T) {
	t.Parallel()

	if !RefreshActionMatches("", "publish") {
		t.Fatal("RefreshActionMatches() = false, want true when current action is missing")
	}
}

func TestRefreshActionMatchesRejectsDifferentAction(t *testing.T) {
	t.Parallel()

	if RefreshActionMatches("save_draft", "publish") {
		t.Fatal("RefreshActionMatches() = true, want false for different actions")
	}
}

func TestRefreshRequestMatchesTrimsCurrentRequestID(t *testing.T) {
	t.Parallel()

	if !RefreshRequestMatches("  refresh-123  ", "refresh-123") {
		t.Fatal("RefreshRequestMatches() = false, want true after trimming current request id")
	}
}

func TestRefreshRequestMatchesRejectsEmptyCurrentRequestID(t *testing.T) {
	t.Parallel()

	if RefreshRequestMatches("", "refresh-123") {
		t.Fatal("RefreshRequestMatches() = true, want false when current request id is missing")
	}
}
