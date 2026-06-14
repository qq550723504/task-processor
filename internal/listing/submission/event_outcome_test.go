package submission

import (
	"errors"
	"testing"
)

func TestResolveEventOutcomeUsesRecordMetadata(t *testing.T) {
	t.Parallel()

	got := ResolveEventOutcome(&EventRecordState{
		Status:         "success",
		RequestID:      "req-1",
		Phase:          "persist_result",
		RemoteRecordID: "remote-1",
	}, nil, nil, nil)

	if got.Status != "success" || got.RequestID != "req-1" || got.Phase != "persist_result" || got.RemoteRecordID != "remote-1" {
		t.Fatalf("outcome = %+v", got)
	}
}

func TestResolveEventOutcomePrefersExplicitResponseNotes(t *testing.T) {
	t.Parallel()

	got := ResolveEventOutcome(
		&EventRecordState{Status: "success"},
		&ResponseOutcome{ValidationNotes: []string{"explicit"}},
		&ResponseOutcome{ValidationNotes: []string{"fallback"}},
		nil,
	)

	if len(got.ValidationNotes) != 1 || got.ValidationNotes[0] != "explicit" {
		t.Fatalf("validation notes = %+v, want explicit notes", got.ValidationNotes)
	}
}

func TestResolveEventOutcomeFallsBackToRecordResponseNotes(t *testing.T) {
	t.Parallel()

	got := ResolveEventOutcome(
		&EventRecordState{Status: "success"},
		nil,
		&ResponseOutcome{ValidationNotes: []string{"fallback"}},
		nil,
	)

	if len(got.ValidationNotes) != 1 || got.ValidationNotes[0] != "fallback" {
		t.Fatalf("validation notes = %+v, want fallback notes", got.ValidationNotes)
	}
}

func TestResolveEventOutcomeCopiesValidationNotes(t *testing.T) {
	t.Parallel()

	source := []string{"note-1"}
	got := ResolveEventOutcome(nil, &ResponseOutcome{ValidationNotes: source}, nil, nil)
	source[0] = "mutated"

	if len(got.ValidationNotes) != 1 || got.ValidationNotes[0] != "note-1" {
		t.Fatalf("validation notes = %+v, want copied notes", got.ValidationNotes)
	}
}

func TestResolveEventOutcomeSubmitErrorOverridesStatus(t *testing.T) {
	t.Parallel()

	got := ResolveEventOutcome(&EventRecordState{Status: "success"}, nil, nil, errors.New("boom"))
	if got.Status != "failed" || got.ErrorMessage != "boom" {
		t.Fatalf("outcome = %+v, want failed/boom", got)
	}
}
