// Package dataservicesapp composes the bounded data-service ports. Business
// transitions and facts remain in their domain/persistence owners.
package dataservicesapp

import (
	"context"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	jobstore "task-processor/internal/integration/persistence/product/dataacquisition"
	sourcestore "task-processor/internal/integration/persistence/product/sourcing"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/dataacquisition"
	"task-processor/internal/product/sourcing"
)

type publicationAuthority struct {
	live dataacquisition.LiveAccess
	job  dataacquisition.Job
}

func (a publicationAuthority) Authorize(ctx context.Context) (sourcing.PublicationScope, error) {
	if err := a.live.CheckExecution(ctx, dataacquisition.Principal{Scope: a.job.Scope, CredentialID: a.job.CredentialID}, a.job.Funding); err != nil {
		return sourcing.PublicationScope{}, err
	}
	return sourcing.PublicationScope{OrganizationID: a.job.Scope.OrganizationID, ActorID: a.job.Scope.ActorID}, nil
}
func NewProductPublisher(live dataacquisition.LiveAccess) (jobstore.Publisher, error) {
	if live == nil {
		return nil, dataacquisition.ErrUnavailable
	}
	return func(ctx context.Context, tx *gorm.DB, job dataacquisition.Job, item dataacquisition.Item) (collection.Source, error) {
		if job.Scope.Validate() != nil || !collection.ValidID(item.ID) || item.Evidence == nil || item.Evidence.ASIN != item.ASIN || item.Evidence.Site != job.Query.Site {
			return collection.Source{}, dataacquisition.ErrInvalid
		}
		store, err := sourcestore.NewTransactionWriter(tx, newCatalogBridge)
		if err != nil {
			return collection.Source{}, err
		}
		descriptor := sourcing.ProducerDescriptor{Kind: "amazon_data", Version: "v1"}
		producer, err := sourcing.NewInternalProducer(publicationAuthority{live, job}, store, descriptor)
		if err != nil {
			return collection.Source{}, err
		}
		envelope, err := item.Evidence.Envelope(item.ID)
		if err != nil {
			return collection.Source{}, err
		}
		productKey := "amazon-" + collection.StableID(job.Scope.OrganizationID, job.Scope.ActorID, "amazon-product", job.Query.Site, item.ASIN)
		receipt, err := producer.Publish(ctx, sourcing.PublicationCommand{PublicationID: item.ID, Producer: descriptor, ProductKey: productKey, Envelope: envelope})
		if err != nil {
			return collection.Source{}, err
		}
		return collection.Source{ProductKey: receipt.ProductKey, PublicationID: receipt.PublicationID, Version: receipt.CatalogVersion, OperationID: item.ID, Kind: "amazon_data"}, nil
	}, nil
}

// This bridge is only DTO/injection glue. Catalog owns locking, version
// allocation, idempotency and snapshot validation through its existing writer.
type catalogBridge struct{ db *gorm.DB }

func newCatalogBridge(db *gorm.DB) (sourcestore.CatalogBridge, error) {
	if db == nil {
		return nil, sourcing.ErrSourcePublicationUnavailable
	}
	return catalogBridge{db}, nil
}
func (b catalogBridge) Publish(ctx context.Context, p sourcing.AtomicPublication) (sourcestore.CatalogBinding, error) {
	writer, err := catalogstore.NewTransactionWriter(b.db)
	if err != nil {
		return sourcestore.CatalogBinding{}, err
	}
	publisher, err := catalog.NewPublisher(writer)
	if err != nil {
		return sourcestore.CatalogBinding{}, err
	}
	published, err := publisher.Publish(ctx, catalog.PublishRequest{Identity: catalog.SnapshotIdentity{TenantID: p.OrganizationID, ProductKey: p.ProductKey}, PublicationID: p.PublicationID, ExpectedBaseVersion: p.ExpectedBaseVersion, Snapshot: p.Snapshot})
	if err != nil {
		return sourcestore.CatalogBinding{}, err
	}
	raw, err := json.Marshal(published.Snapshot)
	return sourcestore.CatalogBinding{Version: published.Version, PublicationID: published.PublicationID, SnapshotJSON: raw}, err
}
func (b catalogBridge) Read(ctx context.Context, org, key string, version uint64) (sourcestore.CatalogBinding, bool, error) {
	reader, err := catalogstore.NewBoundedSnapshotReader(b.db, sourcing.MaxEncodedSnapshotBytes)
	if err != nil {
		return sourcestore.CatalogBinding{}, false, err
	}
	snapshot, err := reader.GetSnapshot(ctx, catalog.SnapshotIdentity{TenantID: org, ProductKey: key}, version)
	if errors.Is(err, catalog.ErrSnapshotNotReady) {
		return sourcestore.CatalogBinding{}, false, nil
	}
	if err != nil {
		return sourcestore.CatalogBinding{}, false, err
	}
	raw, err := json.Marshal(snapshot.Snapshot)
	return sourcestore.CatalogBinding{Version: snapshot.Version, PublicationID: snapshot.PublicationID, SnapshotJSON: raw}, true, err
}
func (b catalogBridge) LockPublicationSlot(ctx context.Context, org, key, publication string) (bool, error) {
	return catalogstore.LockPublicationSlot(ctx, b.db, catalog.SnapshotIdentity{TenantID: org, ProductKey: key}, publication)
}
