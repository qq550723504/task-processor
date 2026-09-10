// Package productsourcing composes the admitted in-process source publication
// capability. It deliberately registers no public route or provider adapter.
package productsourcing

import (
	"context"
	"encoding/json"
	"errors"

	"gorm.io/gorm"

	"task-processor/internal/authz"
	catalogpersistence "task-processor/internal/integration/persistence/product/catalog"
	sourcingpersistence "task-processor/internal/integration/persistence/product/sourcing"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

// NewInternalProducer composes the real internal producer against one existing
// PostgreSQL database. Schema installation is an explicit separate operation.
func NewInternalProducer(db *gorm.DB, live sourcing.LiveOrganizationAccess, permissions *authz.ListingKitAuthorizer) (*sourcing.InternalProducer, error) {
	store, err := sourcingpersistence.NewRepository(db, newCatalogBridge)
	if err != nil {
		return nil, err
	}
	authorizer, err := sourcing.NewContextAuthorizer(live, permissions)
	if err != nil {
		return nil, err
	}
	return sourcing.NewInternalProducer(authorizer, store, sourcing.ProducerDescriptor{Kind: sourcing.ControlledSnapshotProducerKind, Version: sourcing.ControlledSnapshotProducerVersion})
}

// NewTransactionReader composes the admitted read-only source capability on a
// caller-owned PostgreSQL transaction. The returned reader retains fresh
// authorization and cannot publish source evidence.
func NewTransactionReader(tx *gorm.DB, live sourcing.LiveOrganizationAccess, permissions *authz.ListingKitAuthorizer) (*sourcing.InternalReader, error) {
	store, err := sourcingpersistence.NewTransactionReader(tx, newCatalogBridge)
	if err != nil {
		return nil, err
	}
	authorizer, err := sourcing.NewContextAuthorizer(live, permissions)
	if err != nil {
		return nil, err
	}
	return sourcing.NewInternalReader(authorizer, store)
}

// InstallSchema initializes the two current owners atomically for an empty
// task database. It is never called by NewInternalProducer or request paths.
func InstallSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return sourcing.ErrSourcePublicationUnavailable
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := catalogpersistence.AutoMigrate(tx); err != nil {
			return err
		}
		return sourcingpersistence.InstallSchema(tx)
	})
}

type catalogBridge struct{ db *gorm.DB }

func newCatalogBridge(db *gorm.DB) (sourcingpersistence.CatalogBridge, error) {
	if db == nil {
		return nil, sourcing.ErrSourcePublicationUnavailable
	}
	return &catalogBridge{db: db}, nil
}

func (b *catalogBridge) Publish(ctx context.Context, publication sourcing.AtomicPublication) (sourcingpersistence.CatalogBinding, error) {
	writer, err := catalogpersistence.NewTransactionWriter(b.db)
	if err != nil {
		return sourcingpersistence.CatalogBinding{}, err
	}
	publisher, err := catalog.NewPublisher(writer)
	if err != nil {
		return sourcingpersistence.CatalogBinding{}, err
	}
	published, err := publisher.Publish(ctx, catalog.PublishRequest{
		Identity:            catalog.SnapshotIdentity{TenantID: publication.OrganizationID, ProductKey: publication.ProductKey},
		ExpectedBaseVersion: publication.ExpectedBaseVersion,
		PublicationID:       publication.PublicationID,
		Snapshot:            publication.Snapshot,
	})
	if err != nil {
		return sourcingpersistence.CatalogBinding{}, err
	}
	raw, err := json.Marshal(published.Snapshot)
	if err != nil {
		return sourcingpersistence.CatalogBinding{}, err
	}
	return sourcingpersistence.CatalogBinding{Version: published.Version, PublicationID: published.PublicationID, SnapshotJSON: raw}, nil
}

func (b *catalogBridge) Read(ctx context.Context, organizationID, productKey string, version uint64) (sourcingpersistence.CatalogBinding, bool, error) {
	reader, err := catalogpersistence.NewBoundedSnapshotReader(b.db, sourcing.MaxEncodedSnapshotBytes)
	if err != nil {
		return sourcingpersistence.CatalogBinding{}, false, err
	}
	published, err := reader.GetSnapshot(ctx, catalog.SnapshotIdentity{TenantID: organizationID, ProductKey: productKey}, version)
	if errors.Is(err, catalog.ErrSnapshotNotReady) {
		return sourcingpersistence.CatalogBinding{}, false, nil
	}
	if err != nil {
		return sourcingpersistence.CatalogBinding{}, false, err
	}
	raw, err := json.Marshal(published.Snapshot)
	if err != nil {
		return sourcingpersistence.CatalogBinding{}, false, err
	}
	return sourcingpersistence.CatalogBinding{Version: published.Version, PublicationID: published.PublicationID, SnapshotJSON: raw}, true, nil
}

func (b *catalogBridge) LockPublicationSlot(ctx context.Context, organizationID, productKey, publicationID string) (bool, error) {
	return catalogpersistence.LockPublicationSlot(ctx, b.db, catalog.SnapshotIdentity{TenantID: organizationID, ProductKey: productKey}, publicationID)
}

var _ sourcingpersistence.CatalogBridge = (*catalogBridge)(nil)
