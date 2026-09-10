package review

import (
	"context"
	"errors"
	"reflect"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

func (s *Service) source(ctx context.Context, reader catalog.VersionedSnapshotReader, org string, in CreateInput) (catalog.PublishedSnapshot, sourcing.SourceEnvelope, error) {
	p, err := reader.GetSnapshot(ctx, catalog.SnapshotIdentity{TenantID: org, ProductKey: in.ProductKey}, in.BaseVersion)
	if err != nil {
		return p, sourcing.SourceEnvelope{}, err
	}
	if p.Identity.TenantID != org || p.Identity.ProductKey != in.ProductKey || p.Version != in.BaseVersion || !ValidKey(p.PublicationID) {
		return p, sourcing.SourceEnvelope{}, ErrConflict
	}
	persisted, err := s.sourceReader.Read(ctx, p.PublicationID)
	if err != nil {
		return p, sourcing.SourceEnvelope{}, mapSourceReadError(err)
	}
	receipt := persisted.Receipt
	if receipt.OrganizationID != p.Identity.TenantID || receipt.ProductKey != p.Identity.ProductKey ||
		receipt.PublicationID != p.PublicationID || receipt.CatalogPublicationID != p.PublicationID ||
		receipt.CatalogVersion != p.Version || !reflect.DeepEqual(persisted.Snapshot, p.Snapshot) {
		return p, sourcing.SourceEnvelope{}, ErrConflict
	}
	return p, persisted.Envelope, nil
}

func mapSourceReadError(err error) error {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, sourcing.ErrPublicationForbidden):
		return ErrForbidden
	case errors.Is(err, sourcing.ErrSourcePublicationConflict):
		return ErrConflict
	default:
		return ErrUnavailable
	}
}
