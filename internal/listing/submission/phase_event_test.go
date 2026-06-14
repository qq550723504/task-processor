package submission

import (
	"errors"
	"testing"
)

func TestResolvePhaseEventStateDefaultsStatusToRunning(t *testing.T) {
	t.Parallel()

	got := ResolvePhaseEventState("", "", "", nil)
	if got.Status != "running" {
		t.Fatalf("status = %q, want running", got.Status)
	}
}

func TestResolvePhaseEventStateUsesExplicitStatus(t *testing.T) {
	t.Parallel()

	got := ResolvePhaseEventState("confirmed", "", "", nil)
	if got.Status != "confirmed" {
		t.Fatalf("status = %q, want confirmed", got.Status)
	}
}

func TestResolvePhaseEventStateUsesExplicitDetail(t *testing.T) {
	t.Parallel()

	got := ResolvePhaseEventState("", "explicit detail", "fallback detail", nil)
	if got.Detail != "explicit detail" {
		t.Fatalf("detail = %q, want explicit detail", got.Detail)
	}
}

func TestResolvePhaseEventStateFallsBackToDefaultDetail(t *testing.T) {
	t.Parallel()

	got := ResolvePhaseEventState("", "", "fallback detail", nil)
	if got.Detail != "fallback detail" {
		t.Fatalf("detail = %q, want fallback detail", got.Detail)
	}
}

func TestResolvePhaseEventStateCopiesErrorMessage(t *testing.T) {
	t.Parallel()

	got := ResolvePhaseEventState("", "", "", errors.New("boom"))
	if got.ErrorMessage != "boom" {
		t.Fatalf("error = %q, want boom", got.ErrorMessage)
	}
}
