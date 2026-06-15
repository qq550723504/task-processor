package submission

import (
	"testing"
	"time"
)

func TestIsActiveAttempt(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	startedAt := now.Add(-10 * time.Minute)
	expiredAt := now.Add(-time.Minute)

	if !IsActiveAttempt(RecoveryLeaseState{
		CurrentAction:     "publish",
		CurrentRequestID:  "req-1",
		CurrentPhase:      "submit_remote",
		InFlightStartedAt: &startedAt,
	}, "publish", now, 15*time.Minute) {
		t.Fatal("expected active in-flight attempt")
	}
	if IsActiveAttempt(RecoveryLeaseState{
		CurrentAction:     "publish",
		CurrentRequestID:  "req-1",
		CurrentPhase:      "submit_remote",
		InFlightStartedAt: &startedAt,
		LeaseExpiresAt:    &expiredAt,
	}, "publish", now, 15*time.Minute) {
		t.Fatal("expired lease should not be active")
	}
	if IsActiveAttempt(RecoveryLeaseState{
		CurrentAction:    "publish",
		CurrentRequestID: "req-1",
		CurrentPhase:     "submit_remote",
	}, "publish", now, 15*time.Minute) {
		t.Fatal("missing start time should not be active")
	}
}

func TestNeedsRemoteRecovery(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	startedAt := now.Add(-16 * time.Minute)
	leaseExpiresAt := now.Add(-time.Minute)
	recoverable := map[string]struct{}{"submit_remote": {}, "persist_result": {}}

	if !NeedsRemoteRecovery(RecoveryLeaseState{
		CurrentAction:     "publish",
		CurrentRequestID:  "req-1",
		CurrentPhase:      "submit_remote",
		InFlightStartedAt: &startedAt,
	}, "publish", now, 15*time.Minute, recoverable) {
		t.Fatal("expected stale in-flight attempt to need recovery")
	}
	if !NeedsRemoteRecovery(RecoveryLeaseState{
		CurrentAction:    "publish",
		CurrentRequestID: "req-1",
		CurrentPhase:     "submit_remote",
		LeaseExpiresAt:   &leaseExpiresAt,
	}, "publish", now, 15*time.Minute, recoverable) {
		t.Fatal("expected expired lease to need recovery")
	}
	if NeedsRemoteRecovery(RecoveryLeaseState{
		CurrentAction:    "publish",
		CurrentRequestID: "req-1",
		CurrentPhase:     "validate",
		LeaseExpiresAt:   &leaseExpiresAt,
	}, "publish", now, 15*time.Minute, recoverable) {
		t.Fatal("validate phase should not need remote recovery")
	}
	if NeedsRemoteRecovery(RecoveryLeaseState{
		CurrentAction:    "save_draft",
		CurrentRequestID: "req-1",
		CurrentPhase:     "submit_remote",
		LeaseExpiresAt:   &leaseExpiresAt,
	}, "publish", now, 15*time.Minute, recoverable) {
		t.Fatal("mismatched action should not need recovery")
	}
}

func TestNeedsRequestScopedRemoteRecovery(t *testing.T) {
	t.Parallel()

	if !NeedsRequestScopedRemoteRecovery("req-1", "validate", "req-1", "submit_remote", false) {
		t.Fatal("pre-remote current phase should need remote recovery for the same request")
	}
	if !NeedsRequestScopedRemoteRecovery("req-1", "submit_remote", "req-1", "submit_remote", true) {
		t.Fatal("persisted response should need remote recovery confirmation for the same request")
	}
	if NeedsRequestScopedRemoteRecovery("req-1", "submit_remote", "req-1", "submit_remote", false) {
		t.Fatal("in-flight remote submit without persisted response should not force request-scoped recovery")
	}
	if NeedsRequestScopedRemoteRecovery(" req-1 ", "submit_remote", "other", "submit_remote", true) {
		t.Fatal("mismatched request should not need request-scoped recovery")
	}
}
