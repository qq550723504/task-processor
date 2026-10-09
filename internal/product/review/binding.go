package review

import (
	"context"
	"errors"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

func appliedLookup(tx Tx) AppliedPublicationLookup {
	lookup, _ := tx.(AppliedPublicationLookup)
	return lookup
}

func (s *Service) source(ctx context.Context, reader catalog.VersionedSnapshotReader, sourceReader SourcePublicationReader, scope Scope, in CreateInput, transactionLookup ...AppliedPublicationLookup) (catalog.PublishedSnapshot, sourcing.SourceEnvelope, error) {
	p, err := reader.GetSnapshot(ctx, catalog.SnapshotIdentity{TenantID: scope.Org, ProductKey: in.ProductKey}, in.BaseVersion)
	if err != nil {
		return p, sourcing.SourceEnvelope{}, err
	}
	if p.Identity.TenantID != scope.Org || p.Identity.ProductKey != in.ProductKey || p.Version != in.BaseVersion || !ValidKey(p.PublicationID) {
		return p, sourcing.SourceEnvelope{}, ErrConflict
	}
	if sourceReader == nil {
		return p, sourcing.SourceEnvelope{}, ErrUnavailable
	}
	lookup, _ := s.store.(AppliedPublicationLookup)
	if len(transactionLookup) > 0 {
		lookup = transactionLookup[0]
	}
	lineage, err := ResolveAppliedSource(ctx, scope, p, reader, sourceReader, lookup)
	if err != nil {
		return p, sourcing.SourceEnvelope{}, err
	}
	return p, lineage.Source.Envelope, nil
}

func mapSourceReadError(err error) error {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, sourcing.ErrPublicationForbidden):
		return ErrForbidden
	case errors.Is(err, sourcing.ErrSourcePublicationNotFound):
		return ErrNotFound
	case errors.Is(err, sourcing.ErrSourcePublicationConflict):
		return ErrConflict
	default:
		return ErrUnavailable
	}
}
