package imageagent

import (
	"context"
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
	Published                  SlotExecutionResult
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
	Published              SlotExecutionResult
}

type SlotEffectV3BlockTransition struct {
	Reservation SlotEffectV3Reservation
	Phase       SlotEffectV3Phase
	Code        string
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
