package submission

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewExecutionReservationBindsStableIdentityAndPayload(t *testing.T) {
	now := time.Date(2026, 9, 10, 1, 2, 3, 456000000, time.UTC)
	command := AcquireExecutionCommand{
		Scope:        ExecutionScope{OrganizationID: "org-1"},
		IntentKey:    "submit-product-7",
		Target:       ExecutionTarget{Platform: "SHEIN", StoreID: "store-1", SubjectID: "listing-7"},
		Action:       "SAVE_DRAFT",
		Payload:      []byte(`{"title":"shirt"}`),
		ClaimOwnerID: "worker-1",
		Lease:        5 * time.Minute,
	}

	reservation, err := NewExecutionReservation(command, "019938d8-b580-7d04-90f0-2f69c118eb49", "claim-token", now)
	require.NoError(t, err)
	require.Equal(t, "org-1", reservation.Attempt.OrganizationID)
	require.Equal(t, "shein", reservation.Attempt.Target.Platform)
	require.Equal(t, "save_draft", reservation.Attempt.Action)
	require.Equal(t, ExecutionClaimed, reservation.Attempt.Status)
	require.Equal(t, now.Add(5*time.Minute), reservation.Attempt.LeaseExpiresAt)
	require.Regexp(t, `^[0-9a-f]{64}$`, reservation.Attempt.PayloadFingerprint)
	require.Regexp(t, `^subk1_v1_[0-9a-f]{64}$`, reservation.Attempt.ProviderExecutionKey)
	require.Regexp(t, `^[0-9a-f]{64}$`, reservation.ClaimTokenHash)

	same, err := NewExecutionReservation(command, "019938d8-b580-7d04-90f0-2f69c118eb50", "another-token", now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, reservation.Attempt.PayloadFingerprint, same.Attempt.PayloadFingerprint)
	require.Equal(t, reservation.Attempt.ProviderExecutionKey, same.Attempt.ProviderExecutionKey)

	changed := command
	changed.Payload = []byte(`{"title":"different"}`)
	other, err := NewExecutionReservation(changed, "019938d8-b580-7d04-90f0-2f69c118eb51", "third-token", now)
	require.NoError(t, err)
	require.NotEqual(t, reservation.Attempt.PayloadFingerprint, other.Attempt.PayloadFingerprint)
	require.Equal(t, reservation.Attempt.ProviderExecutionKey, other.Attempt.ProviderExecutionKey)
}

func TestNewExecutionReservationCanonicalizesComputedLeaseExpiration(t *testing.T) {
	now := time.Date(2026, 9, 10, 1, 2, 3, 456789123, time.FixedZone("test", 8*60*60))
	command := AcquireExecutionCommand{
		Scope:        ExecutionScope{OrganizationID: "org-1"},
		IntentKey:    "submit-product-lease-precision",
		Target:       ExecutionTarget{Platform: "shein", StoreID: "store-1", SubjectID: "listing-lease-precision"},
		Action:       "save_draft",
		Payload:      []byte(`{"title":"shirt"}`),
		ClaimOwnerID: "worker-1",
		Lease:        5*time.Minute + 789*time.Nanosecond,
	}

	reservation, err := NewExecutionReservation(command, "019938d8-b580-7d04-90f0-2f69c118eb49", "claim-token", now)
	require.NoError(t, err)
	require.Equal(t, now.UTC().Truncate(time.Microsecond), reservation.Attempt.CreatedAt)
	require.Equal(t, now.UTC().Truncate(time.Microsecond).Add(command.Lease).Truncate(time.Microsecond), reservation.Attempt.LeaseExpiresAt)
}

func TestValidateExecutionReplayRequiresExactImmutableIntent(t *testing.T) {
	base := executionAttemptFixture(t, ExecutionClaimed)
	require.NoError(t, ValidateExecutionReplay(base, base))

	mutations := []struct {
		name   string
		change func(*ExecutionAttempt)
	}{
		{"payload", func(v *ExecutionAttempt) { v.PayloadFingerprint = digestExecutionValue("different") }},
		{"target", func(v *ExecutionAttempt) { v.Target.SubjectID = "listing-other" }},
		{"action", func(v *ExecutionAttempt) { v.Action = "publish" }},
		{"provider key", func(v *ExecutionAttempt) { v.ProviderExecutionKey = "subk1_v1_" + digestExecutionValue("other") }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			candidate := base
			tc.change(&candidate)
			require.ErrorIs(t, ValidateExecutionReplay(base, candidate), ErrExecutionIntentConflict)
		})
	}
}

func TestClaimedAttemptTransitionsConservatively(t *testing.T) {
	base := executionAttemptFixture(t, ExecutionClaimed)
	now := base.CreatedAt.Add(time.Minute)

	unknown, err := TransitionExecutionToUnknown(base, UnknownResponseLost, now)
	require.NoError(t, err)
	require.Equal(t, ExecutionOutcomeUnknown, unknown.Status)
	require.Nil(t, unknown.FinishedAt)

	for _, reason := range []UnknownReason{UnknownResponseLost, UnknownLeaseExpired, UnknownExecutionCancelled} {
		got, transitionErr := TransitionExecutionToUnknown(base, reason, now)
		require.NoError(t, transitionErr)
		require.Equal(t, ExecutionOutcomeUnknown, got.Status)
	}

	_, err = TransitionExecutionToUnknown(base, UnknownReason("generic_failure"), now)
	require.ErrorIs(t, err, ErrExecutionInvalid)
}

func TestClaimCompletionAndUnknownResolutionEvidenceGate(t *testing.T) {
	claimed := executionAttemptFixture(t, ExecutionClaimed)
	now := claimed.CreatedAt.Add(time.Minute)
	providerResponse := ExecutionEvidence{
		Kind:        EvidenceProviderResponse,
		Outcome:     ExecutionSucceeded,
		Reference:   "provider-request-1",
		Fingerprint: digestExecutionValue("provider-response"),
		ObservedAt:  now,
	}
	succeeded, err := CompleteClaimedExecution(claimed, providerResponse, now)
	require.NoError(t, err)
	require.Equal(t, ExecutionSucceeded, succeeded.Status)
	require.NotNil(t, succeeded.FinishedAt)

	duplicate, err := CompleteClaimedExecution(succeeded, providerResponse, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, succeeded, duplicate)

	conflicting := providerResponse
	conflicting.Fingerprint = digestExecutionValue("different-response")
	_, err = CompleteClaimedExecution(succeeded, conflicting, now)
	require.ErrorIs(t, err, ErrExecutionIntentConflict)

	unknown, err := TransitionExecutionToUnknown(claimed, UnknownResponseLost, now)
	require.NoError(t, err)
	_, err = CompleteClaimedExecution(unknown, providerResponse, now)
	require.ErrorIs(t, err, ErrExecutionInvalidTransition)

	readBack := providerResponse
	readBack.Kind = EvidenceProviderReadBack
	resolved, err := ResolveUnknownExecution(unknown, readBack, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, ExecutionSucceeded, resolved.Status)

	manualCancel := ExecutionEvidence{
		Kind:         EvidenceManualResolution,
		Outcome:      ExecutionCancelled,
		Reference:    "incident-42",
		Fingerprint:  digestExecutionValue("no-provider-side-effect"),
		Reason:       "operator verified that the provider never received the request",
		AuthorizedBy: "operator-1",
		ObservedAt:   now,
	}
	cancelled, err := ResolveUnknownExecution(unknown, manualCancel, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, ExecutionCancelled, cancelled.Status)

	missingAuthorization := manualCancel
	missingAuthorization.AuthorizedBy = ""
	_, err = ResolveUnknownExecution(unknown, missingAuthorization, now)
	require.ErrorIs(t, err, ErrExecutionEvidenceRequired)

	readBackCancel := manualCancel
	readBackCancel.Kind = EvidenceProviderReadBack
	readBackCancel.AuthorizedBy = ""
	_, err = ResolveUnknownExecution(unknown, readBackCancel, now)
	require.ErrorIs(t, err, ErrExecutionEvidenceRequired)
}

func TestExpiredClaimCannotComplete(t *testing.T) {
	claimed := executionAttemptFixture(t, ExecutionClaimed)
	evidence := ExecutionEvidence{
		Kind:        EvidenceProviderResponse,
		Outcome:     ExecutionSucceeded,
		Reference:   "late-response",
		Fingerprint: digestExecutionValue("late-response"),
		ObservedAt:  claimed.LeaseExpiresAt.Add(time.Second),
	}
	_, err := CompleteClaimedExecution(claimed, evidence, evidence.ObservedAt)
	require.ErrorIs(t, err, ErrExecutionClaimRejected)
}

func TestTerminalReplayCanonicalizesEvidenceTimestamp(t *testing.T) {
	claimed := executionAttemptFixture(t, ExecutionClaimed)
	now := claimed.CreatedAt.Add(time.Minute).Add(789 * time.Nanosecond)
	evidence := ExecutionEvidence{
		Kind: EvidenceProviderResponse, Outcome: ExecutionSucceeded,
		Reference: "provider-request-1", Fingerprint: digestExecutionValue("provider-response"), ObservedAt: now,
	}
	completed, err := CompleteClaimedExecution(claimed, evidence, now)
	require.NoError(t, err)
	require.Equal(t, now.UTC().Truncate(time.Microsecond), completed.Evidence.ObservedAt)
	replayed, err := CompleteClaimedExecution(completed, evidence, now.Add(time.Second))
	require.NoError(t, err)
	require.Equal(t, completed, replayed)
}

func TestExecutionEvidenceReasonCharacterBoundaries(t *testing.T) {
	type transition func(ExecutionAttempt, ExecutionEvidence, time.Time) (ExecutionAttempt, error)
	tests := []struct {
		name           string
		status         ExecutionStatus
		evidence       ExecutionEvidence
		transition     transition
		reasonRequired bool
	}{
		{
			name: "provider response failure", status: ExecutionClaimed,
			evidence:   ExecutionEvidence{Kind: EvidenceProviderResponse, Outcome: ExecutionFailedDefinitive},
			transition: CompleteClaimedExecution, reasonRequired: true,
		},
		{
			name: "provider response success optional reason", status: ExecutionClaimed,
			evidence:   ExecutionEvidence{Kind: EvidenceProviderResponse, Outcome: ExecutionSucceeded},
			transition: CompleteClaimedExecution,
		},
		{
			name: "provider readback failure", status: ExecutionOutcomeUnknown,
			evidence:   ExecutionEvidence{Kind: EvidenceProviderReadBack, Outcome: ExecutionFailedDefinitive},
			transition: ResolveUnknownExecution, reasonRequired: true,
		},
		{
			name: "provider readback success optional reason", status: ExecutionOutcomeUnknown,
			evidence:   ExecutionEvidence{Kind: EvidenceProviderReadBack, Outcome: ExecutionSucceeded},
			transition: ResolveUnknownExecution,
		},
		{
			name: "manual cancellation", status: ExecutionOutcomeUnknown,
			evidence:   ExecutionEvidence{Kind: EvidenceManualResolution, Outcome: ExecutionCancelled, AuthorizedBy: "operator-1"},
			transition: ResolveUnknownExecution, reasonRequired: true,
		},
	}
	validReasons := []struct {
		name   string
		reason string
	}{
		{name: "ASCII within boundary", reason: strings.Repeat("a", 511)},
		{name: "ASCII at boundary", reason: strings.Repeat("a", 512)},
		{name: "multibyte within boundary", reason: strings.Repeat("界", 511)},
		{name: "multibyte at boundary", reason: strings.Repeat("界", 512)},
		{name: "content with surrounding Unicode whitespace", reason: "\t\u00a0verified\u3000\n"},
	}
	invalidReasons := []struct {
		name   string
		reason string
	}{
		{name: "ASCII over boundary", reason: strings.Repeat("a", 513)},
		{name: "multibyte over boundary", reason: strings.Repeat("界", 513)},
		{name: "invalid UTF-8", reason: string([]byte{'o', 'k', 0xff})},
		{name: "PostgreSQL NUL", reason: "ok\x00"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, reasonCase := range validReasons {
				t.Run(reasonCase.name, func(t *testing.T) {
					attempt := executionAttemptFixture(t, tc.status)
					now := attempt.CreatedAt.Add(2 * time.Minute)
					evidence := tc.evidence
					evidence.Reference = "evidence-1"
					evidence.Fingerprint = digestExecutionValue("evidence-1")
					evidence.Reason = reasonCase.reason
					evidence.ObservedAt = now

					terminal, err := tc.transition(attempt, evidence, now)
					require.NoError(t, err)
					require.Equal(t, reasonCase.reason, terminal.Evidence.Reason)
					require.Equal(t, evidence.Fingerprint, terminal.Evidence.Fingerprint)
					require.Equal(t, evidence.Reference, terminal.Evidence.Reference)
					require.NoError(t, ValidatePersistedExecutionAttempt(terminal))

					replayed, err := tc.transition(terminal, evidence, now.Add(time.Second))
					require.NoError(t, err)
					require.Equal(t, terminal, replayed)
				})
			}

			for _, reasonCase := range invalidReasons {
				t.Run(reasonCase.name, func(t *testing.T) {
					attempt := executionAttemptFixture(t, tc.status)
					now := attempt.CreatedAt.Add(2 * time.Minute)
					evidence := tc.evidence
					evidence.Reference = "evidence-1"
					evidence.Fingerprint = digestExecutionValue("evidence-1")
					evidence.Reason = reasonCase.reason
					evidence.ObservedAt = now

					_, err := tc.transition(attempt, evidence, now)
					require.ErrorIs(t, err, ErrExecutionEvidenceRequired)
					require.Equal(t, tc.status, attempt.Status)

					persistedEvidence := evidence
					persistedEvidence.Reason = "valid reason"
					terminal, err := tc.transition(attempt, persistedEvidence, now)
					require.NoError(t, err)
					corruptedEvidence := *terminal.Evidence
					corruptedEvidence.Reason = reasonCase.reason
					terminal.Evidence = &corruptedEvidence
					require.ErrorIs(t, ValidatePersistedExecutionAttempt(terminal), ErrExecutionUnavailable)
				})
			}

			attempt := executionAttemptFixture(t, tc.status)
			now := attempt.CreatedAt.Add(2 * time.Minute)
			evidence := tc.evidence
			evidence.Reference = "evidence-1"
			evidence.Fingerprint = digestExecutionValue("evidence-1")
			evidence.ObservedAt = now
			_, err := tc.transition(attempt, evidence, now)
			if tc.reasonRequired {
				require.ErrorIs(t, err, ErrExecutionEvidenceRequired)
				for _, reason := range []string{"", " \t\n", "\u0085\u00a0\u1680\u2007\u202f\u3000"} {
					evidence.Reason = reason
					_, err = tc.transition(attempt, evidence, now)
					require.ErrorIs(t, err, ErrExecutionEvidenceRequired)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func executionAttemptFixture(t *testing.T, status ExecutionStatus) ExecutionAttempt {
	t.Helper()
	now := time.Date(2026, 9, 10, 1, 2, 3, 0, time.UTC)
	reservation, err := NewExecutionReservation(AcquireExecutionCommand{
		Scope:        ExecutionScope{OrganizationID: "org-1"},
		IntentKey:    "submit-product-7",
		Target:       ExecutionTarget{Platform: "shein", StoreID: "store-1", SubjectID: "listing-7"},
		Action:       "save_draft",
		Payload:      []byte(`{"title":"shirt"}`),
		ClaimOwnerID: "worker-1",
		Lease:        5 * time.Minute,
	}, "019938d8-b580-7d04-90f0-2f69c118eb49", "claim-token", now)
	require.NoError(t, err)
	attempt := reservation.Attempt
	attempt.FenceEpoch = 1
	if status == ExecutionClaimed {
		return attempt
	}
	if status == ExecutionOutcomeUnknown {
		attempt, err = TransitionExecutionToUnknown(attempt, UnknownResponseLost, now.Add(time.Minute))
		require.NoError(t, err)
		return attempt
	}
	t.Fatal(errors.New("unsupported fixture status"))
	return ExecutionAttempt{}
}
