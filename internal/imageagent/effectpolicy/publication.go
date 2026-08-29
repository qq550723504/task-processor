package effectpolicy

import (
	"reflect"
	"strings"
	"time"

	"task-processor/internal/imageagent"
)

// ClaimPublication decides whether a staged artifact can acquire or replay a
// publication lease. observedAt is supplied by the adapter so policy never
// selects between a process clock and the database clock.
func ClaimPublication(current imageagent.SlotEffectV3Attempt, request imageagent.PublicationClaimRequest, observedAt time.Time) (PublicationClaimDecision, error) {
	request, err := PreflightPublicationClaim(request)
	if err != nil {
		return PublicationClaimDecision{}, err
	}
	attempt := cloneSlotEffectV3Attempt(current)
	if err := validateProviderAttemptReservation(attempt, request.Reservation); err != nil {
		return PublicationClaimDecision{}, err
	}

	if attempt.Phase == imageagent.SlotEffectV3ArtifactStaged {
		attempt.Phase = imageagent.SlotEffectV3PublicationClaimed
		attempt.Publication = imageagent.PublicationClaim{Owner: request.Owner, LeaseExpiresAt: observedAt.Add(request.LeaseDuration), Fence: 1}
		attempt.PublicationFingerprint = request.PublicationFingerprint
		attempt.FinalManifest = cloneFinalManifest(request.FinalManifest)
		return PublicationClaimDecision{EffectDecision: EffectDecision{Attempt: attempt, Changed: true}, Claim: attempt.Publication, Acquired: true}, nil
	}
	if attempt.Phase != imageagent.SlotEffectV3PublicationClaimed && attempt.Phase != imageagent.SlotEffectV3PublicationComplete {
		return PublicationClaimDecision{}, imageagent.ErrRevisionConflict
	}
	normalizedCurrent, err := imageagent.NormalizeFinalManifest(attempt.FinalManifest)
	if err != nil {
		return PublicationClaimDecision{}, err
	}
	attempt.FinalManifest = cloneFinalManifest(normalizedCurrent)
	if attempt.PublicationFingerprint != request.PublicationFingerprint || !reflect.DeepEqual(attempt.FinalManifest, request.FinalManifest) {
		return PublicationClaimDecision{}, imageagent.ErrRevisionConflict
	}
	decision := PublicationClaimDecision{EffectDecision: EffectDecision{Attempt: attempt}, Claim: attempt.Publication}
	if attempt.Phase == imageagent.SlotEffectV3PublicationComplete || observedAt.Before(attempt.Publication.LeaseExpiresAt) {
		return decision, nil
	}
	decision.Attempt.Publication.Owner = request.Owner
	decision.Attempt.Publication.Fence++
	decision.Attempt.Publication.LeaseExpiresAt = observedAt.Add(request.LeaseDuration)
	decision.Claim = decision.Attempt.Publication
	decision.Changed = true
	decision.Acquired = true
	return decision, nil
}

// RenewPublication decides whether the current lease owner may extend an
// unexpired lease. Renewal at the exact expiry instant is rejected.
func RenewPublication(current imageagent.SlotEffectV3Attempt, renewal imageagent.PublicationLeaseRenewal, observedAt time.Time) (PublicationLeaseDecision, error) {
	if err := PreflightPublicationLeaseRenewal(renewal); err != nil {
		return PublicationLeaseDecision{}, err
	}
	attempt := cloneSlotEffectV3Attempt(current)
	if err := imageagent.ValidateSlotEffectV3AttemptPolicy(attempt); err != nil {
		return PublicationLeaseDecision{}, err
	}
	if attempt.Identity != renewal.Identity || attempt.Phase != imageagent.SlotEffectV3PublicationClaimed || attempt.Publication.Owner != renewal.Owner || attempt.Publication.Fence != renewal.Fence || !observedAt.Before(attempt.Publication.LeaseExpiresAt) {
		return PublicationLeaseDecision{}, imageagent.ErrRevisionConflict
	}
	attempt.Publication.LeaseExpiresAt = observedAt.Add(renewal.LeaseDuration)
	return PublicationLeaseDecision{EffectDecision: EffectDecision{Attempt: attempt, Changed: true}, Claim: attempt.Publication}, nil
}

// CompletePublication decides whether an owner/fence may bind an ordered,
// normalized published result to the claimed final manifest.
func CompletePublication(current imageagent.SlotEffectV3Attempt, completion imageagent.PublicationCompletion) (EffectDecision, error) {
	completion, err := PreflightPublicationCompletion(completion)
	if err != nil {
		return EffectDecision{}, err
	}
	attempt := cloneSlotEffectV3Attempt(current)
	if err := validateProviderAttemptReservation(attempt, completion.Reservation); err != nil {
		return EffectDecision{}, err
	}
	normalizedManifest, err := imageagent.NormalizeFinalManifest(attempt.FinalManifest)
	if err != nil {
		return EffectDecision{}, err
	}
	attempt.FinalManifest = cloneFinalManifest(normalizedManifest)

	if attempt.Phase == imageagent.SlotEffectV3PublicationComplete {
		normalizedPublished, err := imageagent.NormalizeSlotEffectV3PublishedResult(attempt.Published)
		if err != nil {
			return EffectDecision{}, err
		}
		attempt.Published = clonePublishedResult(normalizedPublished)
		if samePublicationCompletion(attempt, completion) {
			return EffectDecision{Attempt: attempt}, nil
		}
		return EffectDecision{}, imageagent.ErrRevisionConflict
	}
	if attempt.Phase != imageagent.SlotEffectV3PublicationClaimed || attempt.Publication.Owner != completion.Owner || attempt.Publication.Fence != completion.Fence || attempt.PublicationFingerprint != completion.PublicationFingerprint {
		return EffectDecision{}, imageagent.ErrRevisionConflict
	}
	if err := imageagent.ValidateSlotEffectV3Completion(completion.Published, attempt.FinalManifest, completion.ResultFingerprint); err != nil {
		return EffectDecision{}, err
	}
	attempt.Phase = imageagent.SlotEffectV3PublicationComplete
	attempt.ResultFingerprint = completion.ResultFingerprint
	attempt.Published = clonePublishedResult(completion.Published)
	return EffectDecision{Attempt: attempt, Changed: true}, nil
}

// PreflightPublicationClaim validates and normalizes a claim command without
// consulting persisted state, preserving adapter validation precedence.
func PreflightPublicationClaim(request imageagent.PublicationClaimRequest) (imageagent.PublicationClaimRequest, error) {
	normalized, err := imageagent.NormalizeFinalManifest(request.FinalManifest)
	if err != nil {
		return imageagent.PublicationClaimRequest{}, err
	}
	request.FinalManifest = cloneFinalManifest(normalized)
	if err := validateProviderReservation(request.Reservation); err != nil {
		return imageagent.PublicationClaimRequest{}, err
	}
	if strings.TrimSpace(request.Owner) == "" || request.LeaseDuration <= 0 || strings.TrimSpace(request.PublicationFingerprint) == "" {
		return imageagent.PublicationClaimRequest{}, imageagent.ErrValidation
	}
	return request, nil
}

// PreflightPublicationLeaseRenewal validates a renewal command without
// consulting persisted state.
func PreflightPublicationLeaseRenewal(renewal imageagent.PublicationLeaseRenewal) error {
	identity := renewal.Identity
	if strings.TrimSpace(identity.TenantID) == "" || strings.TrimSpace(identity.OwnerUserID) == "" || strings.TrimSpace(identity.RunID) == "" {
		return imageagent.ErrRunNotFound
	}
	if identity.PlanRevision <= 0 || strings.TrimSpace(identity.SlotID) == "" || identity.Attempt <= 0 || strings.TrimSpace(renewal.Owner) == "" || renewal.Fence <= 0 || renewal.LeaseDuration <= 0 {
		return imageagent.ErrValidation
	}
	return nil
}

// PreflightPublicationCompletion validates and normalizes a completion
// command without consulting persisted state.
func PreflightPublicationCompletion(completion imageagent.PublicationCompletion) (imageagent.PublicationCompletion, error) {
	normalized, err := imageagent.NormalizeSlotEffectV3PublishedResult(completion.Published)
	if err != nil {
		return imageagent.PublicationCompletion{}, err
	}
	completion.Published = clonePublishedResult(normalized)
	if err := validateProviderReservation(completion.Reservation); err != nil {
		return imageagent.PublicationCompletion{}, err
	}
	if strings.TrimSpace(completion.Owner) == "" || completion.Fence <= 0 || strings.TrimSpace(completion.PublicationFingerprint) == "" || strings.TrimSpace(completion.ResultFingerprint) == "" || completion.Published.SlotID != completion.Reservation.Identity.SlotID || completion.Published.Attempt != completion.Reservation.Identity.Attempt {
		return imageagent.PublicationCompletion{}, imageagent.ErrValidation
	}
	return completion, nil
}

func samePublicationCompletion(attempt imageagent.SlotEffectV3Attempt, completion imageagent.PublicationCompletion) bool {
	return attempt.Publication.Owner == completion.Owner && attempt.Publication.Fence == completion.Fence && attempt.PublicationFingerprint == completion.PublicationFingerprint && attempt.ResultFingerprint == completion.ResultFingerprint && reflect.DeepEqual(attempt.Published, completion.Published)
}

func cloneFinalManifest(manifest imageagent.FinalManifest) imageagent.FinalManifest {
	manifest.Assets = clonePublishedAssetRefs(manifest.Assets)
	return manifest
}

func clonePublishedResult(result imageagent.SlotEffectV3PublishedResult) imageagent.SlotEffectV3PublishedResult {
	result.Candidates = clonePublishedCandidates(result.Candidates)
	return result
}
