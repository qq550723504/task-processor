package imageagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

type SlotEffectV3Phase string

const (
	SlotEffectV3ProviderClaimed       SlotEffectV3Phase = "provider_claimed"
	SlotEffectV3ProviderNotDispatched SlotEffectV3Phase = "provider_not_dispatched"
	SlotEffectV3StagingPrepared       SlotEffectV3Phase = "staging_prepared"
	SlotEffectV3ArtifactStaged        SlotEffectV3Phase = "artifact_staged"
	SlotEffectV3PublicationClaimed    SlotEffectV3Phase = "publication_claimed"
	SlotEffectV3PublicationComplete   SlotEffectV3Phase = "publication_complete"
	SlotEffectV3ProviderUnknown       SlotEffectV3Phase = "provider_outcome_unknown"
	SlotEffectV3StagingUnknown        SlotEffectV3Phase = "staging_outcome_unknown"
	SlotEffectV3PublicationUnknown    SlotEffectV3Phase = "publication_outcome_unknown"
	SlotEffectV3RecoveryBlocked       SlotEffectV3Phase = "recovery_blocked"
)

const (
	SlotProviderOutcomeUnknownCode    = "slot_provider_outcome_unknown"
	SlotProviderNotDispatchedCode     = "slot_provider_not_dispatched"
	SlotStagingOutcomeUnknownCode     = "slot_staging_outcome_unknown"
	SlotPublicationOutcomeUnknownCode = "slot_publication_outcome_unknown"
	SlotRecoveryBlockedCode           = "recovery_blocked"
	SlotEffectPhaseInvalidCode        = "slot_effect_phase_invalid"
	SlotEffectPolicyInvalidCode       = "slot_effect_policy_invalid"
	BudgetExhaustedCode               = "budget_exhausted"
	BudgetQuoteUnavailableCode        = "budget_quote_unavailable"
	BudgetElapsedCode                 = "budget_elapsed"
	WorkflowLifecycleElapsedCode      = "workflow_lifecycle_elapsed"
)

type SlotEffectV3BlockedPolicy struct {
	Phase            SlotEffectV3Phase
	Code             string
	PermittedActions []Action
}

func SlotEffectV3BlockedPolicyFor(phase SlotEffectV3Phase, code string) (SlotEffectV3BlockedPolicy, error) {
	var policy SlotEffectV3BlockedPolicy
	switch phase {
	case SlotEffectV3ProviderUnknown:
		policy = SlotEffectV3BlockedPolicy{Phase: phase, Code: SlotProviderOutcomeUnknownCode, PermittedActions: []Action{ActionCancel}}
	case SlotEffectV3StagingUnknown:
		policy = SlotEffectV3BlockedPolicy{Phase: phase, Code: SlotStagingOutcomeUnknownCode, PermittedActions: []Action{ActionEditPlan, ActionRetrySlot, ActionCancel}}
	case SlotEffectV3PublicationUnknown:
		policy = SlotEffectV3BlockedPolicy{Phase: phase, Code: SlotPublicationOutcomeUnknownCode, PermittedActions: []Action{ActionEditPlan, ActionCancel}}
	case SlotEffectV3RecoveryBlocked:
		policy = SlotEffectV3BlockedPolicy{Phase: phase, Code: SlotRecoveryBlockedCode, PermittedActions: []Action{ActionCancel}}
	default:
		return SlotEffectV3BlockedPolicy{}, fmt.Errorf("%w: unsupported v3 blocked phase %q", ErrInvalidPersistedPolicy, phase)
	}
	if code != policy.Code {
		return SlotEffectV3BlockedPolicy{}, fmt.Errorf("%w: phase %q requires code %q", ErrInvalidPersistedPolicy, phase, policy.Code)
	}
	policy.PermittedActions = append([]Action(nil), policy.PermittedActions...)
	return policy, nil
}

func SlotEffectV3BlockedPolicyForCode(code string) (SlotEffectV3BlockedPolicy, bool) {
	var phase SlotEffectV3Phase
	switch code {
	case SlotProviderNotDispatchedCode:
		return SlotEffectV3BlockedPolicy{Code: code, PermittedActions: []Action{ActionEditPlan, ActionRetrySlot, ActionCancel}}, true
	case "recovery_requested", "recovery_start_failed":
		return SlotEffectV3BlockedPolicy{Code: code, PermittedActions: []Action{ActionCancel}}, true
	case SlotProviderOutcomeUnknownCode:
		phase = SlotEffectV3ProviderUnknown
	case SlotStagingOutcomeUnknownCode:
		phase = SlotEffectV3StagingUnknown
	case SlotPublicationOutcomeUnknownCode:
		phase = SlotEffectV3PublicationUnknown
	case SlotRecoveryBlockedCode:
		phase = SlotEffectV3RecoveryBlocked
	case SlotEffectPhaseInvalidCode, SlotEffectPolicyInvalidCode:
		return SlotEffectV3BlockedPolicy{Code: code, PermittedActions: []Action{ActionCancel}}, true
	case BudgetExhaustedCode:
		return SlotEffectV3BlockedPolicy{Code: code, PermittedActions: []Action{ActionEditPlan, ActionCancel}}, true
	case BudgetQuoteUnavailableCode, BudgetElapsedCode, WorkflowLifecycleElapsedCode:
		return SlotEffectV3BlockedPolicy{Code: code, PermittedActions: []Action{ActionCancel}}, true
	default:
		if strings.HasPrefix(code, "slot_effect_") {
			return SlotEffectV3BlockedPolicy{Code: SlotEffectPolicyInvalidCode, PermittedActions: []Action{ActionCancel}}, true
		}
		return SlotEffectV3BlockedPolicy{}, false
	}
	policy, err := SlotEffectV3BlockedPolicyFor(phase, code)
	return policy, err == nil
}

// NormalizeSlotEffectV3BlockCode is used only by the additive v3 projection.
// It preserves known provider-neutral policy codes and collapses every unknown
// v3 classification to an explicit fail-closed policy instead of allowing the
// legacy blocked-run retry fallback to reinterpret it.
func NormalizeSlotEffectV3BlockCode(code string) string {
	policy, ok := SlotEffectV3BlockedPolicyForCode(strings.TrimSpace(code))
	if !ok {
		return SlotEffectPolicyInvalidCode
	}
	return policy.Code
}

// ValidateSlotEffectV3AttemptPolicy fails closed on unknown phases and on any
// phase/code mismatch. It is shared by repositories and the activity replay
// interpreter so persisted authorization cannot be silently reclassified.
func ValidateSlotEffectV3AttemptPolicy(attempt SlotEffectV3Attempt) error {
	switch attempt.Phase {
	case SlotEffectV3ProviderUnknown, SlotEffectV3StagingUnknown, SlotEffectV3PublicationUnknown, SlotEffectV3RecoveryBlocked:
		_, err := SlotEffectV3BlockedPolicyFor(attempt.Phase, attempt.BlockedCode)
		return err
	case SlotEffectV3ProviderClaimed, SlotEffectV3ProviderNotDispatched, SlotEffectV3StagingPrepared, SlotEffectV3ArtifactStaged, SlotEffectV3PublicationClaimed, SlotEffectV3PublicationComplete:
		if attempt.BlockedCode != "" {
			return fmt.Errorf("%w: executable phase %q cannot carry blocked code", ErrInvalidPersistedPolicy, attempt.Phase)
		}
		return nil
	default:
		return fmt.Errorf("%w: unsupported v3 phase %q", ErrInvalidPersistedPolicy, attempt.Phase)
	}
}

type SlotEffectV3Reservation struct {
	Identity         SlotExternalEffectIdentity
	IdempotencyKey   string
	InputFingerprint string
	Policy           BudgetPolicy
	Quote            SlotUsageQuote
}

type SlotBudgetStatus string

const (
	SlotBudgetReserved  SlotBudgetStatus = "reserved"
	SlotBudgetCommitted SlotBudgetStatus = "committed"
	SlotBudgetReleased  SlotBudgetStatus = "released"
	SlotBudgetUnknown   SlotBudgetStatus = "unknown"
)

type SlotEffectV3Attempt struct {
	Identity                   SlotExternalEffectIdentity
	IdempotencyKey             string
	InputFingerprint           string
	Phase                      SlotEffectV3Phase
	StagingManifest            StagingManifest
	StagingManifestFingerprint string
	Publication                PublicationClaim
	PublicationFingerprint     string
	FinalManifest              FinalManifest
	ResultFingerprint          string
	Published                  SlotEffectV3PublishedResult
	BlockedCode                string
	// RecoveryPhase records the safe, pre-block phase when bounded recovery
	// exhausts. It lets an explicit owner-scoped redrive resume finalization
	// instead of treating recovery_blocked as a terminal no-op.
	RecoveryPhase SlotEffectV3Phase
	// CorruptionMarker records a deterministic, non-sensitive marker for a
	// persisted payload that could not be decoded. It is retained after the
	// effect is durably fail-closed so recovery never needs the original JSON.
	CorruptionMarker string
	BudgetStatus     SlotBudgetStatus
	Policy           BudgetPolicy
	Quote            SlotUsageQuote
	Receipt          SlotUsageReceipt
}

type PublicationClaim struct {
	Owner          string
	LeaseExpiresAt time.Time
	Fence          int64
}

type PublicationClaimRequest struct {
	Reservation            SlotEffectV3Reservation
	Owner                  string
	LeaseDuration          time.Duration
	PublicationFingerprint string
	FinalManifest          FinalManifest
}

type PublicationLeaseRenewal struct {
	Identity      SlotExternalEffectIdentity
	Owner         string
	Fence         int64
	LeaseDuration time.Duration
}

type PublicationCompletion struct {
	Reservation            SlotEffectV3Reservation
	Owner                  string
	Fence                  int64
	PublicationFingerprint string
	ResultFingerprint      string
	Published              SlotEffectV3PublishedResult
}

type SlotEffectV3BlockTransition struct {
	Reservation SlotEffectV3Reservation
	Phase       SlotEffectV3Phase
	Code        string
	Owner       string
	Fence       int64
}

// SlotEffectV3PublishedResult is the allowlisted persisted v3 result. It is
// separate from the frozen v2 SlotExecutionResult wire contract.
type SlotEffectV3PublishedResult struct {
	SlotID     string                       `json:"slot_id"`
	Attempt    int                          `json:"attempt"`
	Candidates []SlotEffectV3AssetCandidate `json:"candidates"`
}

type SlotEffectV3AssetCandidate struct {
	AssetID       string               `json:"asset_id"`
	SourceAssetID string               `json:"source_asset_id"`
	DurableAsset  DurableAssetIdentity `json:"durable_asset"`
	Width         int                  `json:"width,omitempty"`
	Height        int                  `json:"height,omitempty"`
	Operations    []string             `json:"operations,omitempty"`
}

// NewSlotEffectV3PublishedResult is the explicit adapter from an in-process
// executor result. Legacy URL and arbitrary metadata are not eligible for v3
// persistence and therefore fail closed.
func NewSlotEffectV3PublishedResult(result SlotExecutionResult) (SlotEffectV3PublishedResult, error) {
	candidates := make([]SlotEffectV3AssetCandidate, len(result.Candidates))
	for index, candidate := range result.Candidates {
		if candidate.URL != "" || len(candidate.Metadata) != 0 {
			return SlotEffectV3PublishedResult{}, ErrValidation
		}
		candidates[index] = SlotEffectV3AssetCandidate{
			AssetID: candidate.AssetID, SourceAssetID: candidate.SourceAssetID, DurableAsset: candidate.DurableAsset,
			Width: candidate.Width, Height: candidate.Height, Operations: append([]string(nil), candidate.Operations...),
		}
	}
	return NormalizeSlotEffectV3PublishedResult(SlotEffectV3PublishedResult{SlotID: result.SlotID, Attempt: result.Attempt, Candidates: candidates})
}

func NormalizeSlotEffectV3PublishedResult(result SlotEffectV3PublishedResult) (SlotEffectV3PublishedResult, error) {
	if result.SlotID == "" || result.SlotID != strings.TrimSpace(result.SlotID) || result.Attempt <= 0 || len(result.Candidates) == 0 {
		return SlotEffectV3PublishedResult{}, ErrValidation
	}
	seen := make(map[string]struct{}, len(result.Candidates))
	for index, candidate := range result.Candidates {
		if candidate.AssetID == "" || candidate.AssetID != strings.TrimSpace(candidate.AssetID) || candidate.SourceAssetID == "" || candidate.SourceAssetID != strings.TrimSpace(candidate.SourceAssetID) {
			return SlotEffectV3PublishedResult{}, ErrValidation
		}
		identity, err := NormalizeDurableAssetIdentity(candidate.DurableAsset)
		if err != nil {
			return SlotEffectV3PublishedResult{}, err
		}
		operations, err := NormalizeArtifactOperations(candidate.Operations)
		if err != nil {
			return SlotEffectV3PublishedResult{}, err
		}
		if _, ok := seen[candidate.AssetID]; ok {
			return SlotEffectV3PublishedResult{}, ErrValidation
		}
		seen[candidate.AssetID] = struct{}{}
		candidate.DurableAsset = identity
		candidate.Operations = operations
		result.Candidates[index] = candidate
	}
	return result, nil
}

func SlotEffectV3PublishedResultFingerprint(result SlotEffectV3PublishedResult) (string, error) {
	normalized, err := NormalizeSlotEffectV3PublishedResult(result)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

// ValidateSlotEffectV3Completion binds a published result to the exact ordered
// final manifest. Every final reference is consumed once, at the same index,
// with the same durable key, hash, and source lineage.
func ValidateSlotEffectV3Completion(result SlotEffectV3PublishedResult, manifest FinalManifest, fingerprint string) error {
	normalizedResult, err := NormalizeSlotEffectV3PublishedResult(result)
	if err != nil {
		return err
	}
	normalizedManifest, err := NormalizeFinalManifest(manifest)
	if err != nil {
		return err
	}
	if len(normalizedResult.Candidates) != len(normalizedManifest.Assets) {
		return ErrRevisionConflict
	}
	consumed := make(map[string]struct{}, len(normalizedResult.Candidates))
	for index, candidate := range normalizedResult.Candidates {
		asset := normalizedManifest.Assets[index]
		identity := candidate.DurableAsset.ObjectKey + "\x00" + candidate.DurableAsset.SHA256
		if _, duplicate := consumed[identity]; duplicate {
			return ErrRevisionConflict
		}
		consumed[identity] = struct{}{}
		if candidate.DurableAsset.ObjectKey != asset.ObjectKey || candidate.DurableAsset.SHA256 != asset.SHA256 || candidate.SourceAssetID != asset.SourceAssetID || (slotEffectV3CandidateMetadataPresent(candidate) && (candidate.Width != asset.Width || candidate.Height != asset.Height || !reflect.DeepEqual(candidate.Operations, asset.Operations))) {
			return ErrRevisionConflict
		}
	}
	expected, err := SlotEffectV3PublishedResultFingerprint(normalizedResult)
	if err != nil {
		return err
	}
	if fingerprint != expected {
		return ErrRevisionConflict
	}
	return nil
}

func slotEffectV3CandidateMetadataPresent(candidate SlotEffectV3AssetCandidate) bool {
	return candidate.Width != 0 || candidate.Height != 0 || candidate.Operations != nil
}

type SlotExternalEffectV3Repository interface {
	ReserveSlotProviderV3(context.Context, SlotEffectV3Reservation) (SlotEffectV3Attempt, bool, error)
	PrepareSlotStagingV3(context.Context, SlotEffectV3Reservation, StagingManifest) (SlotEffectV3Attempt, error)
	CommitSlotStagedV3(context.Context, SlotEffectV3Reservation, string) (SlotEffectV3Attempt, error)
	ClaimSlotPublicationV3(context.Context, PublicationClaimRequest) (SlotEffectV3Attempt, PublicationClaim, bool, error)
	RenewSlotPublicationV3(context.Context, PublicationLeaseRenewal) (PublicationClaim, error)
	CompleteSlotPublicationV3(context.Context, PublicationCompletion) (SlotEffectV3Attempt, error)
	BlockSlotEffectV3(context.Context, SlotEffectV3BlockTransition) (SlotEffectV3Attempt, error)
	SettleSlotProviderV3(context.Context, SlotEffectV3Reservation, SlotUsageReceipt) (SlotEffectV3Attempt, error)
	RecordSlotProviderNotDispatchedV3(context.Context, SlotEffectV3Reservation) (SlotEffectV3Attempt, error)
	ReleaseSlotProviderBudgetV3(context.Context, SlotEffectV3Reservation) (SlotEffectV3Attempt, error)
	MarkSlotProviderBudgetUnknownV3(context.Context, SlotEffectV3Reservation) (SlotEffectV3Attempt, error)
	GetSlotExternalEffectV3(context.Context, SlotExternalEffectIdentity) (SlotEffectV3Attempt, error)
}

// CorruptSlotEffectV3Repository is an optional repository capability used by
// recovery to atomically fail closed when persisted authorization JSON is
// corrupt. It intentionally remains separate from the execution repository
// contract so existing adapters cannot accidentally authorize a new effect.
type CorruptSlotEffectV3Repository interface {
	BlockCorruptSlotEffectV3(context.Context, SlotExternalEffectIdentity) (SlotEffectV3Attempt, error)
}

// RecoveryBlockedSlotEffectV3Repository is the narrow, explicit-redrive
// capability. It deliberately requires the original reservation identity so
// callers cannot move an external effect across tenant, plan, slot, or attempt.
type RecoveryBlockedSlotEffectV3Repository interface {
	RestoreRecoveryBlockedEffectV3(context.Context, SlotEffectV3Reservation) (SlotEffectV3Attempt, error)
}
