package submission

import (
	"testing"
	"time"
)

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
