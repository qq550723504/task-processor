package effectpolicy

import (
	"strings"

	"task-processor/internal/imageagent"
)

// PreflightReservation preserves command-validation precedence for adapters
// that must reject malformed requests before loading persisted state.
func PreflightReservation(reservation imageagent.SlotEffectV3Reservation) error {
	return validateProviderReservation(reservation)
}

// PreflightBlock preserves command-validation precedence before an adapter
// acquires its lock or transaction.
func PreflightBlock(transition imageagent.SlotEffectV3BlockTransition) error {
	return validateBlockTransition(transition)
}

// Block decides whether an existing effect may enter one of the canonical
// blocked phases. It never mutates caller-owned state.
func Block(current imageagent.SlotEffectV3Attempt, transition imageagent.SlotEffectV3BlockTransition) (EffectDecision, error) {
	if err := validateBlockTransition(transition); err != nil {
		return EffectDecision{}, err
	}
	attempt := cloneSlotEffectV3Attempt(current)
	if err := imageagent.ValidateSlotEffectV3AttemptPolicy(attempt); err != nil {
		return EffectDecision{}, err
	}
	if !sameRecoveryReservation(attempt, transition.Reservation) || attempt.Phase == imageagent.SlotEffectV3PublicationComplete {
		return EffectDecision{}, imageagent.ErrRevisionConflict
	}
	if attempt.Phase == transition.Phase && attempt.BlockedCode == transition.Code {
		if transition.Phase != imageagent.SlotEffectV3PublicationUnknown ||
			(attempt.Publication.Owner == transition.Owner && attempt.Publication.Fence == transition.Fence) {
			return EffectDecision{Attempt: attempt}, nil
		}
		return EffectDecision{}, imageagent.ErrRevisionConflict
	}
	if transition.Phase != imageagent.SlotEffectV3RecoveryBlocked && isBlockedPhase(attempt.Phase) {
		return EffectDecision{}, imageagent.ErrRevisionConflict
	}
	if !canBlock(attempt.Phase, transition.Phase) {
		return EffectDecision{}, imageagent.ErrRevisionConflict
	}
	if transition.Phase == imageagent.SlotEffectV3PublicationUnknown &&
		(attempt.Publication.Owner != transition.Owner || attempt.Publication.Fence != transition.Fence) {
		return EffectDecision{}, imageagent.ErrRevisionConflict
	}
	if transition.Phase == imageagent.SlotEffectV3RecoveryBlocked {
		attempt.RecoveryPhase = attempt.Phase
	}
	attempt.Phase = transition.Phase
	attempt.BlockedCode = transition.Code
	return EffectDecision{Attempt: attempt, Changed: true}, nil
}

// RestoreRecoveryBlocked decides whether explicit owner-scoped recovery may
// resume a safe finalization phase. Provider-dispatch phases are deliberately
// absent from the allowlist.
func RestoreRecoveryBlocked(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation) (EffectDecision, error) {
	if err := validateProviderReservation(reservation); err != nil {
		return EffectDecision{}, err
	}
	attempt := cloneSlotEffectV3Attempt(current)
	if err := imageagent.ValidateSlotEffectV3AttemptPolicy(attempt); err != nil {
		return EffectDecision{}, err
	}
	if !sameRecoveryReservation(attempt, reservation) || attempt.CorruptionMarker != "" ||
		attempt.Phase != imageagent.SlotEffectV3RecoveryBlocked || !isRedrivableRecoveryPhase(attempt.RecoveryPhase) {
		return EffectDecision{}, imageagent.ErrRevisionConflict
	}
	attempt.Phase = attempt.RecoveryPhase
	attempt.RecoveryPhase = ""
	attempt.BlockedCode = recoveryBlockedCode(attempt.Phase)
	return EffectDecision{Attempt: attempt, Changed: true}, nil
}

// FailClosedCorrupt converts storage-supplied deterministic corrupt evidence
// into the canonical non-executable recovery block. It does not decode or
// reconstruct authorization/provider data and never records a redrive phase.
func FailClosedCorrupt(identity imageagent.SlotExternalEffectIdentity, marker string, current *imageagent.SlotEffectV3Attempt) (EffectDecision, error) {
	if err := validateRecoveryIdentity(identity); err != nil {
		return EffectDecision{}, err
	}
	if strings.TrimSpace(marker) == "" {
		return EffectDecision{}, imageagent.ErrCorruptPersistedEffect
	}
	if current == nil {
		return EffectDecision{Attempt: imageagent.SlotEffectV3Attempt{
			Identity: identity, Phase: imageagent.SlotEffectV3RecoveryBlocked,
			BlockedCode: imageagent.SlotRecoveryBlockedCode, CorruptionMarker: marker,
		}, Changed: true}, nil
	}

	attempt := cloneSlotEffectV3Attempt(*current)
	if attempt.Identity != identity {
		return EffectDecision{}, imageagent.ErrRevisionConflict
	}
	if attempt.Phase == imageagent.SlotEffectV3RecoveryBlocked && strings.TrimSpace(attempt.CorruptionMarker) == "" {
		return EffectDecision{}, imageagent.ErrCorruptPersistedEffect
	}
	if attempt.CorruptionMarker != "" && attempt.CorruptionMarker != marker {
		return EffectDecision{}, imageagent.ErrRevisionConflict
	}
	if attempt.Phase == imageagent.SlotEffectV3RecoveryBlocked &&
		attempt.BlockedCode == imageagent.SlotRecoveryBlockedCode &&
		attempt.RecoveryPhase == "" && attempt.CorruptionMarker == marker {
		return EffectDecision{Attempt: attempt}, nil
	}
	attempt.Phase = imageagent.SlotEffectV3RecoveryBlocked
	attempt.BlockedCode = imageagent.SlotRecoveryBlockedCode
	attempt.RecoveryPhase = ""
	attempt.CorruptionMarker = marker
	return EffectDecision{Attempt: attempt, Changed: true}, nil
}

func validateBlockTransition(transition imageagent.SlotEffectV3BlockTransition) error {
	if err := validateProviderReservation(transition.Reservation); err != nil {
		return err
	}
	if !isBlockedPhase(transition.Phase) || strings.TrimSpace(transition.Code) == "" {
		return imageagent.ErrValidation
	}
	if _, err := imageagent.SlotEffectV3BlockedPolicyFor(transition.Phase, transition.Code); err != nil {
		return err
	}
	if transition.Phase == imageagent.SlotEffectV3PublicationUnknown &&
		(strings.TrimSpace(transition.Owner) == "" || transition.Fence <= 0) {
		return imageagent.ErrValidation
	}
	return nil
}

func isBlockedPhase(phase imageagent.SlotEffectV3Phase) bool {
	return phase == imageagent.SlotEffectV3ProviderUnknown || phase == imageagent.SlotEffectV3StagingUnknown ||
		phase == imageagent.SlotEffectV3PublicationUnknown || phase == imageagent.SlotEffectV3ReviewRequired || phase == imageagent.SlotEffectV3ReviewTransportRequired || phase == imageagent.SlotEffectV3RecoveryBlocked
}

func isRedrivableRecoveryPhase(phase imageagent.SlotEffectV3Phase) bool {
	return phase == imageagent.SlotEffectV3StagingPrepared || phase == imageagent.SlotEffectV3ArtifactStaged ||
		phase == imageagent.SlotEffectV3PublicationClaimed || phase == imageagent.SlotEffectV3ProviderUnknown ||
		phase == imageagent.SlotEffectV3StagingUnknown || phase == imageagent.SlotEffectV3PublicationUnknown
}

func recoveryBlockedCode(phase imageagent.SlotEffectV3Phase) string {
	switch phase {
	case imageagent.SlotEffectV3ProviderUnknown:
		return imageagent.SlotProviderOutcomeUnknownCode
	case imageagent.SlotEffectV3StagingUnknown:
		return imageagent.SlotStagingOutcomeUnknownCode
	case imageagent.SlotEffectV3PublicationUnknown:
		return imageagent.SlotPublicationOutcomeUnknownCode
	default:
		return ""
	}
}

func canBlock(current, blocked imageagent.SlotEffectV3Phase) bool {
	switch blocked {
	case imageagent.SlotEffectV3ProviderUnknown:
		return current == imageagent.SlotEffectV3ProviderClaimed
	case imageagent.SlotEffectV3StagingUnknown:
		return current == imageagent.SlotEffectV3StagingPrepared
	case imageagent.SlotEffectV3PublicationUnknown:
		return current == imageagent.SlotEffectV3PublicationClaimed
	case imageagent.SlotEffectV3ReviewRequired:
		return current == imageagent.SlotEffectV3ProviderClaimed || current == imageagent.SlotEffectV3StagingPrepared
	case imageagent.SlotEffectV3ReviewTransportRequired:
		return current == imageagent.SlotEffectV3ProviderClaimed || current == imageagent.SlotEffectV3StagingPrepared
	case imageagent.SlotEffectV3RecoveryBlocked:
		return current == imageagent.SlotEffectV3ProviderClaimed || current == imageagent.SlotEffectV3ProviderNotDispatched ||
			current == imageagent.SlotEffectV3StagingPrepared || current == imageagent.SlotEffectV3ArtifactStaged ||
			current == imageagent.SlotEffectV3PublicationClaimed || current == imageagent.SlotEffectV3ProviderUnknown ||
			current == imageagent.SlotEffectV3StagingUnknown || current == imageagent.SlotEffectV3PublicationUnknown
	default:
		return false
	}
}

// ResumeReviewRetry moves a reviewer-transport block back to the durable
// staging boundary without changing its effect identity or manifest.
func ResumeReviewRetry(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation) (EffectDecision, error) {
	if err := validateProviderReservation(reservation); err != nil {
		return EffectDecision{}, err
	}
	attempt := cloneSlotEffectV3Attempt(current)
	if err := validateProviderAttemptReservation(attempt, reservation); err != nil {
		return EffectDecision{}, err
	}
	if attempt.Phase != imageagent.SlotEffectV3ReviewTransportRequired || attempt.BlockedCode != imageagent.SlotReviewTransportRequiredCode || attempt.StagingManifestFingerprint == "" {
		return EffectDecision{}, imageagent.ErrRevisionConflict
	}
	attempt.Phase = imageagent.SlotEffectV3StagingPrepared
	attempt.BlockedCode = ""
	return EffectDecision{Attempt: attempt, Changed: true}, nil
}

func sameRecoveryReservation(attempt imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation) bool {
	return attempt.Identity == reservation.Identity && attempt.IdempotencyKey == reservation.IdempotencyKey &&
		attempt.InputFingerprint == reservation.InputFingerprint && attempt.Quote.Fingerprint == reservation.Quote.Fingerprint
}

func validateRecoveryIdentity(identity imageagent.SlotExternalEffectIdentity) error {
	if strings.TrimSpace(identity.TenantID) == "" || strings.TrimSpace(identity.OwnerUserID) == "" || strings.TrimSpace(identity.RunID) == "" {
		return imageagent.ErrRunNotFound
	}
	if identity.PlanRevision <= 0 || strings.TrimSpace(identity.SlotID) == "" || identity.Attempt <= 0 {
		return imageagent.ErrValidation
	}
	return nil
}
