package review

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

type exactCatalogReaderStub struct {
	published catalog.PublishedSnapshot
	err       error
}

func (stub exactCatalogReaderStub) GetSnapshot(context.Context, catalog.SnapshotIdentity, uint64) (catalog.PublishedSnapshot, error) {
	return stub.published, stub.err
}

type sourcePublicationReaderStub struct {
	requested string
	value     sourcing.PersistedPublication
	err       error
}

func (stub *sourcePublicationReaderStub) Read(_ context.Context, publicationID string) (sourcing.PersistedPublication, error) {
	stub.requested = publicationID
	return stub.value, stub.err
}

func TestSourceDerivesPublicationFromExactCatalogVersion(t *testing.T) {
	snapshot := catalog.ProductSnapshot{Title: "source title", Brand: "brand"}
	published := catalog.PublishedSnapshot{
		Identity:      catalog.SnapshotIdentity{TenantID: "org", ProductKey: "product"},
		Version:       7,
		PublicationID: "source-publication",
		Snapshot:      snapshot,
	}
	source := &sourcePublicationReaderStub{value: sourcing.PersistedPublication{
		Receipt: sourcing.PublicationReceipt{
			OrganizationID:       "org",
			PublicationID:        "source-publication",
			ProductKey:           "product",
			CatalogVersion:       7,
			CatalogPublicationID: "source-publication",
		},
		Envelope: sourcing.SourceEnvelope{Identity: sourcing.SourceIdentity{SourceType: sourcing.SourceTypeManualImport, SourcePlatform: "fixture", SourceID: "source", SourceVersion: "v1"}},
		Snapshot: snapshot,
	}}
	service := &Service{reader: exactCatalogReaderStub{published: published}, sourceReader: source}

	base, envelope, err := service.source(context.Background(), service.reader, service.sourceReader, "org", CreateInput{ProductKey: "product", BaseVersion: 7})
	require.NoError(t, err)
	require.Equal(t, "source-publication", source.requested)
	require.Equal(t, published, base)
	require.Equal(t, source.value.Envelope, envelope)
}

func TestSourceRejectsCatalogAndSourceBindingMismatch(t *testing.T) {
	published := catalog.PublishedSnapshot{
		Identity:      catalog.SnapshotIdentity{TenantID: "org", ProductKey: "product"},
		Version:       7,
		PublicationID: "source-publication",
		Snapshot:      catalog.ProductSnapshot{Title: "catalog title"},
	}
	tests := []struct {
		name   string
		mutate func(*sourcing.PersistedPublication)
	}{
		{"organization", func(value *sourcing.PersistedPublication) { value.Receipt.OrganizationID = "other" }},
		{"product", func(value *sourcing.PersistedPublication) { value.Receipt.ProductKey = "other" }},
		{"version", func(value *sourcing.PersistedPublication) { value.Receipt.CatalogVersion = 8 }},
		{"publication", func(value *sourcing.PersistedPublication) { value.Receipt.CatalogPublicationID = "other" }},
		{"snapshot", func(value *sourcing.PersistedPublication) { value.Snapshot.Title = "other" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := sourcing.PersistedPublication{Receipt: sourcing.PublicationReceipt{
				OrganizationID: "org", PublicationID: "source-publication", ProductKey: "product",
				CatalogVersion: 7, CatalogPublicationID: "source-publication",
			}, Snapshot: published.Snapshot}
			test.mutate(&value)
			service := &Service{reader: exactCatalogReaderStub{published: published}, sourceReader: &sourcePublicationReaderStub{value: value}}
			_, _, err := service.source(context.Background(), service.reader, service.sourceReader, "org", CreateInput{ProductKey: "product", BaseVersion: 7})
			require.ErrorIs(t, err, ErrConflict)
		})
	}
}

func TestSourceMapsExactBoundaryFailures(t *testing.T) {
	published := catalog.PublishedSnapshot{
		Identity: catalog.SnapshotIdentity{TenantID: "org", ProductKey: "product"}, Version: 7, PublicationID: "source-publication",
	}
	tests := []struct {
		name string
		err  error
		want error
	}{
		{"catalog missing", catalog.ErrSnapshotNotReady, catalog.ErrSnapshotNotReady},
		{"source forbidden", sourcing.ErrPublicationForbidden, ErrForbidden},
		{"source publication missing", sourcing.ErrSourcePublicationNotFound, ErrNotFound},
		{"source corrupt", sourcing.ErrSourcePublicationStateInvalid, ErrUnavailable},
		{"source dependency", sourcing.ErrSourcePublicationUnavailable, ErrUnavailable},
		{"canceled", context.Canceled, context.Canceled},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			catalogReader := exactCatalogReaderStub{published: published}
			sourceReader := &sourcePublicationReaderStub{}
			if errors.Is(test.err, catalog.ErrSnapshotNotReady) {
				catalogReader.err = test.err
			} else {
				sourceReader.err = test.err
			}
			service := &Service{reader: catalogReader, sourceReader: sourceReader}
			_, _, err := service.source(context.Background(), service.reader, service.sourceReader, "org", CreateInput{ProductKey: "product", BaseVersion: 7})
			require.ErrorIs(t, err, test.want)
		})
	}
}
