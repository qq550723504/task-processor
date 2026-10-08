package review

import (
	"context"
	"reflect"
	"strings"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/enrichment"
	"task-processor/internal/product/sourcing"
)

const MaxAppliedTitleLineage = 64

// This narrow lookup never admits an administrator's other owners. It reads
// existing applied facts, including from the current Review transaction.
type AppliedPublicationLookup interface {
	ReadAppliedPublication(context.Context, Scope, string, uint64, string) (Record, error)
}
type AppliedTitleReference struct {
	ProposalID        string `json:"proposalId"`
	BaseVersion       uint64 `json:"baseVersion,string"`
	BasePublicationID string `json:"basePublicationId"`
	Version           uint64 `json:"version,string"`
	PublicationID     string `json:"publicationId"`
}
type SourceLineage struct {
	Original catalog.PublishedSnapshot
	Source   sourcing.PersistedPublication
	Applied  []AppliedTitleReference
}
type TitleLineage struct {
	Original catalog.PublishedSnapshot
	Applied  []AppliedTitleReference
}

func ResolveAppliedSnapshot(ctx context.Context, scope Scope, published catalog.PublishedSnapshot, snapshots catalog.VersionedSnapshotReader, lookup AppliedPublicationLookup) (TitleLineage, error) {
	lineage, _, err := resolveAppliedSnapshot(ctx, scope, published, snapshots, lookup)
	return lineage, err
}

// ResolveAppliedSource follows real title-only Apply receipts back to the
// exact SRC-1 publication. It does not create a derived source receipt.
func ResolveAppliedSource(ctx context.Context, scope Scope, published catalog.PublishedSnapshot, snapshots catalog.VersionedSnapshotReader, sources SourcePublicationReader, lookup AppliedPublicationLookup) (SourceLineage, error) {
	var result SourceLineage
	if sources == nil {
		return result, ErrUnavailable
	}
	lineage, applied, err := resolveAppliedSnapshot(ctx, scope, published, snapshots, lookup)
	if err != nil {
		return result, err
	}
	current := lineage.Original
	persisted, err := sources.Read(ctx, current.PublicationID)
	if err != nil {
		return result, mapSourceReadError(err)
	}
	r := persisted.Receipt
	if r.OrganizationID != current.Identity.TenantID || r.ProductKey != current.Identity.ProductKey || r.PublicationID != current.PublicationID || r.CatalogPublicationID != current.PublicationID || r.CatalogVersion != current.Version || !reflect.DeepEqual(persisted.Snapshot, current.Snapshot) {
		return result, ErrConflict
	}
	if len(applied) > 0 {
		evidence, err := enrichment.CanonicalEvidenceID(persisted.Envelope)
		if err != nil {
			return result, err
		}
		for _, record := range applied {
			ids := record.Original.Changes[0].EvidenceIDs
			if len(ids) != 1 || ids[0] != evidence {
				return result, ErrConflict
			}
		}
	}
	return SourceLineage{Original: current, Source: persisted, Applied: lineage.Applied}, ctx.Err()
}

func resolveAppliedSnapshot(ctx context.Context, scope Scope, published catalog.PublishedSnapshot, snapshots catalog.VersionedSnapshotReader, lookup AppliedPublicationLookup) (TitleLineage, []Record, error) {
	var result TitleLineage
	if ctx == nil || ctx.Err() != nil || snapshots == nil || published.Identity.TenantID != scope.Org || !ValidKey(published.Identity.ProductKey) || published.Version == 0 || !ValidKey(published.PublicationID) {
		return result, nil, ErrConflict
	}
	current := published
	var applied []Record
	for strings.HasPrefix(current.PublicationID, "review:") {
		if !ValidKey(scope.Actor) || lookup == nil || len(applied) >= MaxAppliedTitleLineage {
			return result, nil, ErrUnavailable
		}
		private := Scope{Org: scope.Org, Actor: scope.Actor}
		r, err := lookup.ReadAppliedPublication(ctx, private, current.Identity.ProductKey, current.Version, current.PublicationID)
		if err != nil {
			return result, nil, err
		}
		if ValidateStoredRecord(r) != nil || r.Org != scope.Org || r.Owner != scope.Actor || r.State != "applied" || r.Receipt == nil || r.Policy != "title-review-v1" || r.Input.ProductKey != current.Identity.ProductKey || r.Input.BaseVersion >= current.Version || r.Receipt.ProductVersion != current.Version || r.Receipt.PublicationID != current.PublicationID || r.Receipt.ProposalID != r.ID || r.Receipt.Revision != r.Revision || ValidateTitle(r.Title) != nil || len(r.Original.Changes) != 1 || r.Original.Changes[0].Field != "title" {
			return result, nil, ErrConflict
		}
		base, err := snapshots.GetSnapshot(ctx, current.Identity, r.Input.BaseVersion)
		if err != nil {
			return result, nil, err
		}
		if base.Identity != current.Identity || base.Version != r.Input.BaseVersion || base.PublicationID != r.BasePublicationID || base.Snapshot.Title != r.Before || current.Snapshot.Title != r.Title {
			return result, nil, ErrConflict
		}
		expected, err := catalog.CloneProductSnapshot(base.Snapshot)
		if err != nil {
			return result, nil, ErrUnavailable
		}
		expected.Title = r.Title
		if !reflect.DeepEqual(expected, current.Snapshot) {
			return result, nil, ErrConflict
		}
		applied = append(applied, r)
		result.Applied = append(result.Applied, AppliedTitleReference{r.ID, base.Version, base.PublicationID, current.Version, current.PublicationID})
		current = base
		if ctx.Err() != nil {
			return result, nil, ctx.Err()
		}
	}
	result.Original = current
	return result, applied, ctx.Err()
}
