package effectpolicy

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/imageagent"
)

func TestBlockDecisionMatrix(t *testing.T) {
	reservation := providerPolicyReservation()
	phases := []imageagent.SlotEffectV3Phase{
		imageagent.SlotEffectV3ProviderClaimed,
		imageagent.SlotEffectV3ProviderNotDispatched,
		imageagent.SlotEffectV3StagingPrepared,
		imageagent.SlotEffectV3ArtifactStaged,
		imageagent.SlotEffectV3PublicationClaimed,
		imageagent.SlotEffectV3PublicationComplete,
		imageagent.SlotEffectV3ProviderUnknown,
		imageagent.SlotEffectV3StagingUnknown,
		imageagent.SlotEffectV3PublicationUnknown,
		imageagent.SlotEffectV3RecoveryBlocked,
	}
	targets := []struct {
		phase imageagent.SlotEffectV3Phase
		code  string
	}{
		{phase: imageagent.SlotEffectV3ProviderUnknown, code: imageagent.SlotProviderOutcomeUnknownCode},
		{phase: imageagent.SlotEffectV3StagingUnknown, code: imageagent.SlotStagingOutcomeUnknownCode},
		{phase: imageagent.SlotEffectV3PublicationUnknown, code: imageagent.SlotPublicationOutcomeUnknownCode},
		{phase: imageagent.SlotEffectV3RecoveryBlocked, code: imageagent.SlotRecoveryBlockedCode},
	}
	allowed := map[imageagent.SlotEffectV3Phase]map[imageagent.SlotEffectV3Phase]bool{
		imageagent.SlotEffectV3ProviderClaimed:       {imageagent.SlotEffectV3ProviderUnknown: true, imageagent.SlotEffectV3RecoveryBlocked: true},
		imageagent.SlotEffectV3ProviderNotDispatched: {imageagent.SlotEffectV3RecoveryBlocked: true},
		imageagent.SlotEffectV3StagingPrepared:       {imageagent.SlotEffectV3StagingUnknown: true, imageagent.SlotEffectV3RecoveryBlocked: true},
		imageagent.SlotEffectV3ArtifactStaged:        {imageagent.SlotEffectV3RecoveryBlocked: true},
		imageagent.SlotEffectV3PublicationClaimed:    {imageagent.SlotEffectV3PublicationUnknown: true, imageagent.SlotEffectV3RecoveryBlocked: true},
		imageagent.SlotEffectV3ProviderUnknown:       {imageagent.SlotEffectV3ProviderUnknown: true, imageagent.SlotEffectV3RecoveryBlocked: true},
		imageagent.SlotEffectV3StagingUnknown:        {imageagent.SlotEffectV3StagingUnknown: true, imageagent.SlotEffectV3RecoveryBlocked: true},
		imageagent.SlotEffectV3PublicationUnknown:    {imageagent.SlotEffectV3PublicationUnknown: true, imageagent.SlotEffectV3RecoveryBlocked: true},
		imageagent.SlotEffectV3RecoveryBlocked:       {imageagent.SlotEffectV3RecoveryBlocked: true},
	}

	for _, currentPhase := range phases {
		for _, target := range targets {
			name := string(currentPhase) + "/" + string(target.phase)
			t.Run(name, func(t *testing.T) {
				current := recoveryPolicyAttempt(reservation, currentPhase)
				transition := imageagent.SlotEffectV3BlockTransition{Reservation: reservation, Phase: target.phase, Code: target.code}
				if target.phase == imageagent.SlotEffectV3PublicationUnknown {
					transition.Owner = current.Publication.Owner
					transition.Fence = current.Publication.Fence
				}

				decision, err := Block(current, transition)
				if !allowed[currentPhase][target.phase] {
					require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
					return
				}
				require.NoError(t, err)
				require.Equal(t, target.phase, decision.Attempt.Phase)
				require.Equal(t, target.code, decision.Attempt.BlockedCode)
				repeated := currentPhase == target.phase
				require.Equal(t, !repeated, decision.Changed)
				if target.phase == imageagent.SlotEffectV3RecoveryBlocked && !repeated {
					require.Equal(t, currentPhase, decision.Attempt.RecoveryPhase)
				}
			})
		}
	}

	tests := []struct {
		name       string
		current    imageagent.SlotEffectV3Attempt
		transition imageagent.SlotEffectV3BlockTransition
		wantErr    error
	}{
		{
			name:    "phase code mismatch",
			current: recoveryPolicyAttempt(reservation, imageagent.SlotEffectV3ProviderClaimed),
			transition: imageagent.SlotEffectV3BlockTransition{Reservation: reservation, Phase: imageagent.SlotEffectV3ProviderUnknown,
				Code: imageagent.SlotPublicationOutcomeUnknownCode},
			wantErr: imageagent.ErrInvalidPersistedPolicy,
		},
		{
			name:       "unsupported blocked phase",
			current:    recoveryPolicyAttempt(reservation, imageagent.SlotEffectV3ProviderClaimed),
			transition: imageagent.SlotEffectV3BlockTransition{Reservation: reservation, Phase: imageagent.SlotEffectV3ArtifactStaged, Code: imageagent.SlotStagingOutcomeUnknownCode},
			wantErr:    imageagent.ErrValidation,
		},
		{
			name:       "publication unknown requires owner",
			current:    recoveryPolicyAttempt(reservation, imageagent.SlotEffectV3PublicationClaimed),
			transition: imageagent.SlotEffectV3BlockTransition{Reservation: reservation, Phase: imageagent.SlotEffectV3PublicationUnknown, Code: imageagent.SlotPublicationOutcomeUnknownCode, Fence: 7},
			wantErr:    imageagent.ErrValidation,
		},
		{
			name:       "publication unknown requires positive fence",
			current:    recoveryPolicyAttempt(reservation, imageagent.SlotEffectV3PublicationClaimed),
			transition: imageagent.SlotEffectV3BlockTransition{Reservation: reservation, Phase: imageagent.SlotEffectV3PublicationUnknown, Code: imageagent.SlotPublicationOutcomeUnknownCode, Owner: "publisher-a"},
			wantErr:    imageagent.ErrValidation,
		},
		{
			name:       "publication unknown rejects stale owner",
			current:    recoveryPolicyAttempt(reservation, imageagent.SlotEffectV3PublicationClaimed),
			transition: imageagent.SlotEffectV3BlockTransition{Reservation: reservation, Phase: imageagent.SlotEffectV3PublicationUnknown, Code: imageagent.SlotPublicationOutcomeUnknownCode, Owner: "publisher-b", Fence: 7},
			wantErr:    imageagent.ErrRevisionConflict,
		},
		{
			name:    "publication unknown exact repeat rejects stale fence",
			current: recoveryPolicyAttempt(reservation, imageagent.SlotEffectV3PublicationUnknown),
			transition: imageagent.SlotEffectV3BlockTransition{Reservation: reservation, Phase: imageagent.SlotEffectV3PublicationUnknown,
				Code: imageagent.SlotPublicationOutcomeUnknownCode, Owner: "publisher-a", Fence: 8},
			wantErr: imageagent.ErrRevisionConflict,
		},
		{
			name:    "reservation mismatch",
			current: recoveryPolicyAttempt(reservation, imageagent.SlotEffectV3ProviderClaimed),
			transition: func() imageagent.SlotEffectV3BlockTransition {
				conflict := reservation
				conflict.InputFingerprint = "other-input"
				return imageagent.SlotEffectV3BlockTransition{Reservation: conflict, Phase: imageagent.SlotEffectV3ProviderUnknown, Code: imageagent.SlotProviderOutcomeUnknownCode}
			}(),
			wantErr: imageagent.ErrRevisionConflict,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Block(tc.current, tc.transition)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestRestoreRecoveryBlockedDecisionMatrix(t *testing.T) {
	reservation := providerPolicyReservation()
	allowed := map[imageagent.SlotEffectV3Phase]string{
		imageagent.SlotEffectV3StagingPrepared:    "",
		imageagent.SlotEffectV3ArtifactStaged:     "",
		imageagent.SlotEffectV3PublicationClaimed: "",
		imageagent.SlotEffectV3ProviderUnknown:    imageagent.SlotProviderOutcomeUnknownCode,
		imageagent.SlotEffectV3StagingUnknown:     imageagent.SlotStagingOutcomeUnknownCode,
		imageagent.SlotEffectV3PublicationUnknown: imageagent.SlotPublicationOutcomeUnknownCode,
	}
	recoveryPhases := []imageagent.SlotEffectV3Phase{
		"",
		imageagent.SlotEffectV3ProviderClaimed,
		imageagent.SlotEffectV3ProviderNotDispatched,
		imageagent.SlotEffectV3StagingPrepared,
		imageagent.SlotEffectV3ArtifactStaged,
		imageagent.SlotEffectV3PublicationClaimed,
		imageagent.SlotEffectV3PublicationComplete,
		imageagent.SlotEffectV3ProviderUnknown,
		imageagent.SlotEffectV3StagingUnknown,
		imageagent.SlotEffectV3PublicationUnknown,
		imageagent.SlotEffectV3RecoveryBlocked,
	}
	for _, recoveryPhase := range recoveryPhases {
		name := string(recoveryPhase)
		if name == "" {
			name = "missing"
		}
		t.Run(name, func(t *testing.T) {
			current := recoveryPolicyAttempt(reservation, imageagent.SlotEffectV3RecoveryBlocked)
			current.RecoveryPhase = recoveryPhase
			decision, err := RestoreRecoveryBlocked(current, reservation)
			wantCode, ok := allowed[recoveryPhase]
			if !ok {
				require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
				return
			}
			require.NoError(t, err)
			require.True(t, decision.Changed)
			require.Equal(t, recoveryPhase, decision.Attempt.Phase)
			require.Equal(t, wantCode, decision.Attempt.BlockedCode)
			require.Empty(t, decision.Attempt.RecoveryPhase)
		})
	}

	t.Run("only recovery blocked can restore", func(t *testing.T) {
		current := recoveryPolicyAttempt(reservation, imageagent.SlotEffectV3StagingPrepared)
		_, err := RestoreRecoveryBlocked(current, reservation)
		require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
	})
	t.Run("reservation mismatch", func(t *testing.T) {
		current := recoveryPolicyAttempt(reservation, imageagent.SlotEffectV3RecoveryBlocked)
		current.RecoveryPhase = imageagent.SlotEffectV3ArtifactStaged
		conflict := reservation
		conflict.IdempotencyKey = "different-key"
		_, err := RestoreRecoveryBlocked(current, conflict)
		require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
	})
	t.Run("corruption marker rejects otherwise redrivable recovery phases", func(t *testing.T) {
		for _, recoveryPhase := range []imageagent.SlotEffectV3Phase{
			imageagent.SlotEffectV3StagingPrepared,
			imageagent.SlotEffectV3ArtifactStaged,
			imageagent.SlotEffectV3PublicationClaimed,
			imageagent.SlotEffectV3ProviderUnknown,
			imageagent.SlotEffectV3StagingUnknown,
			imageagent.SlotEffectV3PublicationUnknown,
		} {
			t.Run(string(recoveryPhase), func(t *testing.T) {
				current := recoveryPolicyAttempt(reservation, imageagent.SlotEffectV3RecoveryBlocked)
				current.RecoveryPhase = recoveryPhase
				current.CorruptionMarker = "budget_policy_json:sha256:0123456789abcdef"

				_, err := RestoreRecoveryBlocked(current, reservation)
				require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
			})
		}
	})
}

func TestFailClosedCorruptDecisionMatrix(t *testing.T) {
	reservation := providerPolicyReservation()
	identity := reservation.Identity
	marker := "budget_policy_json:sha256:0123456789abcdef"

	tests := []struct {
		name        string
		identity    imageagent.SlotExternalEffectIdentity
		marker      string
		current     *imageagent.SlotEffectV3Attempt
		wantChanged bool
		wantErr     error
	}{
		{name: "stable evidence without decoded attempt", identity: identity, marker: marker, wantChanged: true},
		{name: "stable evidence replaces executable phase", identity: identity, marker: marker, current: recoveryAttemptPointer(recoveryPolicyAttempt(reservation, imageagent.SlotEffectV3ProviderClaimed)), wantChanged: true},
		{name: "same fail closed repeat", identity: identity, marker: marker, current: recoveryAttemptPointer(func() imageagent.SlotEffectV3Attempt {
			attempt := recoveryPolicyAttempt(reservation, imageagent.SlotEffectV3RecoveryBlocked)
			attempt.CorruptionMarker = marker
			return attempt
		}()), wantChanged: false},
		{name: "same marker with recovery phase is canonicalized", identity: identity, marker: marker, current: recoveryAttemptPointer(func() imageagent.SlotEffectV3Attempt {
			attempt := recoveryPolicyAttempt(reservation, imageagent.SlotEffectV3RecoveryBlocked)
			attempt.RecoveryPhase = imageagent.SlotEffectV3PublicationClaimed
			attempt.CorruptionMarker = marker
			return attempt
		}()), wantChanged: true},
		{name: "different marker cannot replace stable evidence", identity: identity, marker: marker + "-other", current: recoveryAttemptPointer(func() imageagent.SlotEffectV3Attempt {
			attempt := recoveryPolicyAttempt(reservation, imageagent.SlotEffectV3RecoveryBlocked)
			attempt.CorruptionMarker = marker
			return attempt
		}()), wantErr: imageagent.ErrRevisionConflict},
		{name: "missing marker", identity: identity, wantErr: imageagent.ErrCorruptPersistedEffect},
		{name: "existing fail closed row missing marker", identity: identity, marker: marker, current: recoveryAttemptPointer(recoveryPolicyAttempt(reservation, imageagent.SlotEffectV3RecoveryBlocked)), wantErr: imageagent.ErrCorruptPersistedEffect},
		{name: "same marker with mismatched code is canonicalized", identity: identity, marker: marker, current: recoveryAttemptPointer(func() imageagent.SlotEffectV3Attempt {
			attempt := recoveryPolicyAttempt(reservation, imageagent.SlotEffectV3RecoveryBlocked)
			attempt.BlockedCode = imageagent.SlotProviderOutcomeUnknownCode
			attempt.CorruptionMarker = marker
			return attempt
		}()), wantChanged: true},
		{name: "decoded identity mismatch", identity: identity, marker: marker, current: recoveryAttemptPointer(func() imageagent.SlotEffectV3Attempt {
			attempt := recoveryPolicyAttempt(reservation, imageagent.SlotEffectV3ProviderClaimed)
			attempt.Identity.SlotID = "other-slot"
			return attempt
		}()), wantErr: imageagent.ErrRevisionConflict},
		{name: "missing run scope", identity: imageagent.SlotExternalEffectIdentity{}, marker: marker, wantErr: imageagent.ErrRunNotFound},
		{name: "invalid attempt", identity: func() imageagent.SlotExternalEffectIdentity {
			invalid := identity
			invalid.Attempt = 0
			return invalid
		}(), marker: marker, wantErr: imageagent.ErrValidation},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decision, err := FailClosedCorrupt(tc.identity, tc.marker, tc.current)
			if tc.wantErr != nil {
				require.True(t, errors.Is(err, tc.wantErr), "FailClosedCorrupt() error = %v, want errors.Is(_, %v)", err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantChanged, decision.Changed)
			require.Equal(t, tc.identity, decision.Attempt.Identity)
			require.Equal(t, imageagent.SlotEffectV3RecoveryBlocked, decision.Attempt.Phase)
			require.Equal(t, imageagent.SlotRecoveryBlockedCode, decision.Attempt.BlockedCode)
			require.Equal(t, tc.marker, decision.Attempt.CorruptionMarker)
			require.Empty(t, decision.Attempt.RecoveryPhase, "corrupt evidence must never authorize redrive")
		})
	}
}

func TestRecoveryDecisionsDoNotMutateInputs(t *testing.T) {
	reservation := providerPolicyReservation()
	current := recoveryPolicyAttempt(reservation, imageagent.SlotEffectV3PublicationClaimed)
	current.StagingManifest = imageagent.StagingManifest{Assets: []imageagent.StagedAssetRef{{ObjectKey: "staging/asset-a", Operations: []string{"resize"}}}, ProviderMetadata: map[string]string{"provider": "fixture"}}
	current.FinalManifest = imageagent.FinalManifest{Assets: []imageagent.PublishedAssetRef{{ObjectKey: "final/asset-a", Operations: []string{"publish"}}}}
	current.Published = imageagent.SlotEffectV3PublishedResult{Candidates: []imageagent.SlotEffectV3AssetCandidate{{AssetID: "candidate-a"}}}
	current.Quote.Operations = []imageagent.SlotUsageOperation{{Name: "generate"}}
	current.Receipt.ProviderRequestIDs = []string{"request-a"}
	transition := imageagent.SlotEffectV3BlockTransition{Reservation: reservation, Phase: imageagent.SlotEffectV3RecoveryBlocked, Code: imageagent.SlotRecoveryBlockedCode}
	originalCurrent := cloneSlotEffectV3Attempt(current)
	originalTransition := transition
	originalTransition.Reservation.Quote.Operations = append([]imageagent.SlotUsageOperation(nil), transition.Reservation.Quote.Operations...)

	blocked, err := Block(current, transition)
	require.NoError(t, err)
	mutateRecoveryDecision(&blocked.Attempt)
	require.True(t, reflect.DeepEqual(originalCurrent, current), "Block mutated caller-owned current")
	require.True(t, reflect.DeepEqual(originalTransition, transition), "Block mutated caller-owned transition")

	recoveryBlocked := cloneSlotEffectV3Attempt(originalCurrent)
	recoveryBlocked.Phase = imageagent.SlotEffectV3RecoveryBlocked
	recoveryBlocked.BlockedCode = imageagent.SlotRecoveryBlockedCode
	recoveryBlocked.RecoveryPhase = imageagent.SlotEffectV3PublicationClaimed
	originalRecoveryBlocked := cloneSlotEffectV3Attempt(recoveryBlocked)
	restored, err := RestoreRecoveryBlocked(recoveryBlocked, reservation)
	require.NoError(t, err)
	mutateRecoveryDecision(&restored.Attempt)
	require.True(t, reflect.DeepEqual(originalRecoveryBlocked, recoveryBlocked), "RestoreRecoveryBlocked mutated caller-owned current")

	corruptCurrent := cloneSlotEffectV3Attempt(originalCurrent)
	originalCorruptCurrent := cloneSlotEffectV3Attempt(corruptCurrent)
	failClosed, err := FailClosedCorrupt(reservation.Identity, "usage_quote_json:sha256:0123456789abcdef", &corruptCurrent)
	require.NoError(t, err)
	mutateRecoveryDecision(&failClosed.Attempt)
	require.True(t, reflect.DeepEqual(originalCorruptCurrent, corruptCurrent), "FailClosedCorrupt mutated caller-owned current")
}

func recoveryPolicyAttempt(reservation imageagent.SlotEffectV3Reservation, phase imageagent.SlotEffectV3Phase) imageagent.SlotEffectV3Attempt {
	attempt := imageagent.SlotEffectV3Attempt{
		Identity: reservation.Identity, IdempotencyKey: reservation.IdempotencyKey, InputFingerprint: reservation.InputFingerprint,
		Phase: phase, Policy: reservation.Policy, Quote: cloneSlotUsageQuote(reservation.Quote),
		Publication: imageagent.PublicationClaim{Owner: "publisher-a", Fence: 7},
	}
	switch phase {
	case imageagent.SlotEffectV3ProviderUnknown:
		attempt.BlockedCode = imageagent.SlotProviderOutcomeUnknownCode
	case imageagent.SlotEffectV3StagingUnknown:
		attempt.BlockedCode = imageagent.SlotStagingOutcomeUnknownCode
	case imageagent.SlotEffectV3PublicationUnknown:
		attempt.BlockedCode = imageagent.SlotPublicationOutcomeUnknownCode
	case imageagent.SlotEffectV3RecoveryBlocked:
		attempt.BlockedCode = imageagent.SlotRecoveryBlockedCode
	}
	return attempt
}

func recoveryAttemptPointer(attempt imageagent.SlotEffectV3Attempt) *imageagent.SlotEffectV3Attempt {
	return &attempt
}

func mutateRecoveryDecision(attempt *imageagent.SlotEffectV3Attempt) {
	attempt.StagingManifest.Assets[0].Operations[0] = "mutated"
	attempt.StagingManifest.ProviderMetadata["provider"] = "mutated"
	attempt.FinalManifest.Assets[0].Operations[0] = "mutated"
	attempt.Published.Candidates[0].AssetID = "mutated"
	attempt.Quote.Operations[0].Name = "mutated"
	attempt.Receipt.ProviderRequestIDs[0] = "mutated"
}
