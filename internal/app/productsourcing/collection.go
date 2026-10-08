package productsourcing

import (
	"context"
	"gorm.io/gorm"
	"strings"
	"task-processor/internal/authidentity"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	sourcingstore "task-processor/internal/integration/persistence/product/sourcing"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/sourcing"
	"time"
)

// AcquisitionPublicationOption is explicit capability composition. Disabled
// collections do not write or backfill references during publication replay.
type AcquisitionPublicationOption func(*acquisitionPublicationOptions)
type acquisitionPublicationOptions struct{ collection bool }

func WithCollectionPublication() AcquisitionPublicationOption {
	return func(options *acquisitionPublicationOptions) { options.collection = true }
}
func acquisitionPublicationStore(db *gorm.DB, options []AcquisitionPublicationOption) (sourcing.PublicationStore, error) {
	selected := acquisitionPublicationOptions{}
	for _, option := range options {
		if option == nil {
			return nil, collection.ErrUnavailable
		}
		option(&selected)
	}
	if selected.collection {
		return sourcingstore.NewRepositoryWithPublicationObserver(db, newCatalogBridge, newPublicationChargeGuard, func(tx *gorm.DB) (sourcingstore.PublicationObserver, error) {
			return collectionPublicationObserver{tx}, nil
		})
	}
	return sourcingstore.NewRepositoryWithAcquisitionGuard(db, newCatalogBridge, newPublicationChargeGuard)
}

type collectionPublicationObserver struct{ tx *gorm.DB }

func (o collectionPublicationObserver) Published(ctx context.Context, publication sourcing.AtomicPublication, receipt sourcing.PublicationReceipt) error {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	scope := collection.Scope{OrganizationID: receipt.OrganizationID, ActorID: receipt.ActorID, MemberID: identity.EffectiveMemberID}
	operation := strings.TrimPrefix(publication.Envelope.Trace.SourceRunID, "acquisition:")
	if !ok || scope.Validate() != nil || identity.UserID != scope.ActorID || identity.TenantID != scope.OrganizationID || identity.EffectiveOrganizationID != scope.OrganizationID || !time.Now().Before(identity.TokenExpiresAt) || publication.ActorID != scope.ActorID || publication.OrganizationID != scope.OrganizationID || !collection.ValidID(operation) || publication.Envelope.Trace.SourceRunID != "acquisition:"+operation {
		return collection.ErrForbidden
	}
	return collectionstore.AppendPublished(ctx, o.tx, scope, collection.Source{ProductKey: receipt.ProductKey, PublicationID: receipt.PublicationID, Version: receipt.CatalogVersion, OperationID: operation, Kind: "acquisition"}, receipt.PublishedAt)
}

type ownProductWriter struct {
	tx   *gorm.DB
	auth collection.Authorizer
}

func NewOwnProductWriter(tx *gorm.DB, auth collection.Authorizer) (collectionstore.OwnPublisher, error) {
	if tx == nil || auth == nil {
		return nil, collection.ErrUnavailable
	}
	return ownProductWriter{tx: tx, auth: auth}, nil
}

type ownPublicationAuthority struct {
	auth     collection.Authorizer
	expected collection.Scope
}

func (a ownPublicationAuthority) Authorize(ctx context.Context) (sourcing.PublicationScope, error) {
	scope, err := a.auth.Authorize(ctx, collection.PermissionManage)
	if err != nil {
		return sourcing.PublicationScope{}, err
	}
	if scope != a.expected {
		return sourcing.PublicationScope{}, collection.ErrForbidden
	}
	return sourcing.PublicationScope{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID}, nil
}
func (p ownProductWriter) PublishOwn(ctx context.Context, scope collection.Scope, operation string, envelope sourcing.SourceEnvelope) (collection.Source, error) {
	store, err := sourcingstore.NewTransactionWriter(p.tx, newCatalogBridge)
	if err != nil {
		return collection.Source{}, err
	}
	producer := sourcing.ProducerDescriptor{Kind: "own_product", Version: "v1"}
	publisher, err := sourcing.NewInternalProducer(ownPublicationAuthority{auth: p.auth, expected: scope}, store, producer)
	if err != nil {
		return collection.Source{}, err
	}
	zero := uint64(0)
	receipt, err := publisher.Publish(ctx, sourcing.PublicationCommand{PublicationID: operation, ProductKey: "own-" + operation, Producer: producer, ExpectedBaseVersion: &zero, Envelope: envelope})
	if err != nil {
		return collection.Source{}, err
	}
	return collection.Source{ProductKey: receipt.ProductKey, PublicationID: receipt.PublicationID, Version: receipt.CatalogVersion, Kind: "own"}, nil
}
