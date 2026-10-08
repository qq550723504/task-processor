package preparation

import (
	"context"
	"task-processor/internal/authidentity"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
)

type CollectionOwnerAuthority interface {
	AuthorizeOwner(context.Context) (collection.AuthorizedOwner, error)
}
type RetainedSourceRepository interface {
	ReadRetainedSource(context.Context, Scope, string) (SourceItem, error)
}
type SourceSelector struct {
	service   *Service
	owners    CollectionOwnerAuthority
	sources   RetainedSourceRepository
	snapshots catalog.VersionedSnapshotReader
}

func NewSourceSelector(service *Service, owners CollectionOwnerAuthority, sources RetainedSourceRepository, snapshots catalog.VersionedSnapshotReader) (*SourceSelector, error) {
	if service == nil || owners == nil || sources == nil || snapshots == nil {
		return nil, ErrUnavailable
	}
	return &SourceSelector{service, owners, sources, snapshots}, nil
}

type AuthorizedSource struct {
	owner    collection.AuthorizedOwner
	source   SourceItem
	snapshot catalog.PublishedSnapshot
}

func (p AuthorizedSource) Read(ctx context.Context) (Scope, SourceItem, catalog.PublishedSnapshot, error) {
	scope, err := p.owner.Scope(ctx)
	if err != nil || p.snapshot.Identity.TenantID != scope.OrganizationID || p.source.Source.ProductKey != p.snapshot.Identity.ProductKey || p.source.Source.PublicationID != p.snapshot.PublicationID || p.source.Source.Version != p.snapshot.Version {
		return Scope{}, SourceItem{}, catalog.PublishedSnapshot{}, ErrForbidden
	}
	copy := p.snapshot
	copy.Snapshot, err = catalog.CloneProductSnapshot(copy.Snapshot)
	if err != nil {
		return Scope{}, SourceItem{}, catalog.PublishedSnapshot{}, ErrUnavailable
	}
	return scope, p.source, copy, nil
}

// The durable preparation source, including its original Catalog version,
// remains readable after mutable collection membership moves or is archived.
// Both current Supply read and Data read are still required for the old owner.
func (s *SourceSelector) Select(ctx context.Context, id string) (AuthorizedSource, error) {
	if s == nil || ctx == nil {
		return AuthorizedSource{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.service.authorize(ctx, PermissionRead)
	if err != nil {
		return AuthorizedSource{}, err
	}
	if !collection.ValidID(id) {
		return AuthorizedSource{}, ErrInvalid
	}
	owner, err := s.owners.AuthorizeOwner(ctx)
	if err != nil {
		return AuthorizedSource{}, err
	}
	ownerScope, err := owner.Scope(ctx)
	if err != nil || ownerScope != scope {
		return AuthorizedSource{}, ErrForbidden
	}
	source, err := s.sources.ReadRetainedSource(ctx, scope, id)
	if err != nil {
		return AuthorizedSource{}, err
	}
	if source.ID != id || !collection.ValidID(source.PreparationID) || !collection.ValidID(source.CollectionItemID) || source.CollectionRevision < 1 || !authidentity.IsBoundedIdentifier(source.Source.ProductKey) || !authidentity.IsBoundedIdentifier(source.Source.PublicationID) || source.Source.Version == 0 || source.Source.Version > 1<<63-1 {
		return AuthorizedSource{}, ErrUnavailable
	}
	snapshot, err := s.snapshots.GetSnapshot(ctx, catalog.SnapshotIdentity{TenantID: scope.OrganizationID, ProductKey: source.Source.ProductKey}, source.Source.Version)
	if err != nil {
		return AuthorizedSource{}, ErrUnavailable
	}
	proof := AuthorizedSource{owner: owner, source: source, snapshot: snapshot}
	if _, _, _, err = proof.Read(ctx); err != nil {
		return AuthorizedSource{}, err
	}
	return proof, nil
}
