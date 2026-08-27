package imageagent

import (
	"context"
	"strings"
	"time"
)

type SlotEffectV3Phase string

const (
	SlotEffectV3ProviderClaimed     SlotEffectV3Phase = "provider_claimed"
	SlotEffectV3StagingPrepared     SlotEffectV3Phase = "staging_prepared"
	SlotEffectV3ArtifactStaged      SlotEffectV3Phase = "artifact_staged"
	SlotEffectV3PublicationClaimed  SlotEffectV3Phase = "publication_claimed"
	SlotEffectV3PublicationComplete SlotEffectV3Phase = "publication_complete"
	SlotEffectV3ProviderUnknown     SlotEffectV3Phase = "provider_outcome_unknown"
	SlotEffectV3StagingUnknown      SlotEffectV3Phase = "staging_outcome_unknown"
	SlotEffectV3PublicationUnknown  SlotEffectV3Phase = "publication_outcome_unknown"
)

type SlotEffectV3Reservation struct {
	Identity         SlotExternalEffectIdentity
	IdempotencyKey   string
	InputFingerprint string
}

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
		candidates[index] = SlotEffectV3AssetCandidate{AssetID: candidate.AssetID, SourceAssetID: candidate.SourceAssetID, DurableAsset: candidate.DurableAsset}
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
		if _, ok := seen[candidate.AssetID]; ok {
			return SlotEffectV3PublishedResult{}, ErrValidation
		}
		seen[candidate.AssetID] = struct{}{}
		candidate.DurableAsset = identity
		result.Candidates[index] = candidate
	}
	return result, nil
}

type SlotExternalEffectV3Repository interface {
	ReserveSlotProviderV3(context.Context, SlotEffectV3Reservation) (SlotEffectV3Attempt, bool, error)
	PrepareSlotStagingV3(context.Context, SlotEffectV3Reservation, StagingManifest) (SlotEffectV3Attempt, error)
	CommitSlotStagedV3(context.Context, SlotEffectV3Reservation, string) (SlotEffectV3Attempt, error)
	ClaimSlotPublicationV3(context.Context, PublicationClaimRequest) (SlotEffectV3Attempt, PublicationClaim, bool, error)
	RenewSlotPublicationV3(context.Context, PublicationLeaseRenewal) (PublicationClaim, error)
	CompleteSlotPublicationV3(context.Context, PublicationCompletion) (SlotEffectV3Attempt, error)
	BlockSlotEffectV3(context.Context, SlotEffectV3BlockTransition) (SlotEffectV3Attempt, error)
	GetSlotExternalEffectV3(context.Context, SlotExternalEffectIdentity) (SlotEffectV3Attempt, error)
}
