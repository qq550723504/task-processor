package submission

import (
	"testing"
	"time"
)

func TestBuildConfirmRemoteStateUsesDetailAsMessage(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 14, 17, 0, 0, 0, time.UTC)
	state := BuildConfirmRemoteState("confirmed remotely", "", "", now)

	if state.Message != "confirmed remotely" {
		t.Fatalf("message = %q, want confirmed remotely", state.Message)
	}
	if !state.CheckedAt.Equal(now) {
		t.Fatalf("checkedAt = %v, want %v", state.CheckedAt, now)
	}
}

func TestBuildConfirmRemoteStateFallsBackToRecordRemoteRecordID(t *testing.T) {
	t.Parallel()

	state := BuildConfirmRemoteState("confirmed remotely", "", "record-123", time.Now())
	if state.EventRemoteRecordID != "record-123" {
		t.Fatalf("event remote record id = %q, want record-123", state.EventRemoteRecordID)
	}
}

func TestBuildConfirmRemoteStateKeepsExplicitEventRemoteRecordID(t *testing.T) {
	t.Parallel()

	state := BuildConfirmRemoteState("confirmed remotely", "event-record", "record-123", time.Now())
	if state.EventRemoteRecordID != "event-record" {
		t.Fatalf("event remote record id = %q, want event-record", state.EventRemoteRecordID)
	}
}
