package storecenter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

const NoAuthoritativeHistorySourceManifestV1 = "store-service-history/no-authoritative-source/v1"

var ErrInvalidNoAuthoritativeHistoryManifest = errors.New("invalid no-authoritative-history-source manifest")

// NoAuthoritativeHistorySourceManifest is the explicit Product Decision that
// no legacy system ever owned paid Store service periods. The approval fields
// are part of the immutable token persisted on every active Store row.
type NoAuthoritativeHistorySourceManifest struct {
	SchemaVersion     string    `json:"schema_version"`
	DecisionReference string    `json:"decision_reference"`
	ApprovedBy        string    `json:"approved_by"`
	ApprovedAt        time.Time `json:"approved_at"`
}

type NoAuthoritativeHistorySourceResolver struct {
	manifest NoAuthoritativeHistorySourceManifest
	token    string
}

func NewNoAuthoritativeHistorySourceResolver(manifest NoAuthoritativeHistorySourceManifest) (*NoAuthoritativeHistorySourceResolver, error) {
	normalized, token, err := normalizeNoAuthoritativeHistoryManifest(manifest)
	if err != nil {
		return nil, err
	}
	return &NoAuthoritativeHistorySourceResolver{manifest: normalized, token: token}, nil
}

var _ LegacyServiceHistoryResolver = (*NoAuthoritativeHistorySourceResolver)(nil)

func (resolver *NoAuthoritativeHistorySourceResolver) Resolve(ctx context.Context, store StoreSnapshot) (LegacyServiceHistoryResolution, LegacyServiceHistoryFreeze, error) {
	if resolver == nil || resolver.token == "" {
		return LegacyServiceHistoryResolution{}, nil, ErrInvalidNoAuthoritativeHistoryManifest
	}
	if err := ctx.Err(); err != nil {
		return LegacyServiceHistoryResolution{}, nil, err
	}
	if _, err := canonicalUUID(store.ID); err != nil {
		return LegacyServiceHistoryResolution{}, nil, err
	}
	if _, err := validateOpaqueIdentity("organization ID", store.OrganizationID, MaxOrganizationIDBytes); err != nil {
		return LegacyServiceHistoryResolution{}, nil, err
	}
	resolution := LegacyServiceHistoryResolution{
		Status:              HistoryConfirmedAbsent,
		SourceIdentity:      resolver.manifest.DecisionReference,
		SourceSnapshotToken: resolver.token,
	}
	return resolution, noAuthoritativeHistoryFreeze{token: resolver.token}, nil
}

func (resolver *NoAuthoritativeHistorySourceResolver) sourceIdentity() string {
	return resolver.manifest.DecisionReference
}

func (resolver *NoAuthoritativeHistorySourceResolver) snapshotToken() string {
	return resolver.token
}

type noAuthoritativeHistoryFreeze struct{ token string }

func (freeze noAuthoritativeHistoryFreeze) SourceSnapshotToken() string { return freeze.token }

// There is no source and therefore no writer to fence or freeze to release.
// The immutable approved manifest remains the durable handoff proof.
func (noAuthoritativeHistoryFreeze) Handoff(context.Context) error { return nil }
func (noAuthoritativeHistoryFreeze) Release(context.Context) error { return nil }

func normalizeNoAuthoritativeHistoryManifest(manifest NoAuthoritativeHistorySourceManifest) (NoAuthoritativeHistorySourceManifest, string, error) {
	if manifest.SchemaVersion != NoAuthoritativeHistorySourceManifestV1 || manifest.ApprovedAt.IsZero() {
		return NoAuthoritativeHistorySourceManifest{}, "", ErrInvalidNoAuthoritativeHistoryManifest
	}
	decision, err := validateOpaqueIdentity("decision reference", manifest.DecisionReference, 256)
	if err != nil {
		return NoAuthoritativeHistorySourceManifest{}, "", ErrInvalidNoAuthoritativeHistoryManifest
	}
	approver, err := validateOpaqueIdentity("approved by", manifest.ApprovedBy, MaxSubjectBytes)
	if err != nil {
		return NoAuthoritativeHistorySourceManifest{}, "", ErrInvalidNoAuthoritativeHistoryManifest
	}
	manifest.DecisionReference = decision
	manifest.ApprovedBy = approver
	manifest.ApprovedAt = manifest.ApprovedAt.UTC()
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return NoAuthoritativeHistorySourceManifest{}, "", ErrInvalidNoAuthoritativeHistoryManifest
	}
	digest := sha256.Sum256(encoded)
	return manifest, hex.EncodeToString(digest[:]), nil
}
