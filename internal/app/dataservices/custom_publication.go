package dataservicesapp

import (
	"context"
	"gorm.io/gorm"
	"strconv"
	"task-processor/internal/dataservice"
	customstore "task-processor/internal/integration/persistence/dataservice"
	sourcestore "task-processor/internal/integration/persistence/product/sourcing"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/sourcing"
)

// This authorizer consumes the private synchronous delivery proof, after the
// request owner has checked the operator and original applicant in this UoW.
// It never installs a specialist identity as the applicant's identity.
type declaredAuthority struct{ proof dataservice.DeliveryAuthority }

func (a declaredAuthority) Authorize(context.Context) (sourcing.PublicationScope, error) {
	if !a.proof.Valid() {
		return sourcing.PublicationScope{}, dataservice.ErrForbidden
	}
	scope := a.proof.Scope()
	return sourcing.PublicationScope{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID}, nil
}
func NewCustomProductPublisher() (customstore.CustomPublisher, error) {
	return func(ctx context.Context, tx *gorm.DB, a dataservice.DeliveryAuthority, operation string, product collection.OwnProduct) (collection.Source, error) {
		if !a.Valid() || !collection.ValidID(operation) || dataservice.ValidateCustomProducts([]collection.OwnProduct{product}) != nil {
			return collection.Source{}, dataservice.ErrInvalid
		}
		envelope, err := collection.OwnEnvelope(operation, product)
		if err != nil {
			return collection.Source{}, err
		}
		envelope.Identity.SourcePlatform = "custom_dataset"
		envelope.RawReference.ReferenceType = "specialist_declared"
		envelope.Trace.SourceRunID = "custom_dataset:" + operation
		if envelope.ProductCandidate.Attributes == nil {
			envelope.ProductCandidate.Attributes = map[string]string{}
		}
		for key, value := range map[string]string{"data.declaration": "specialist_declared", "data.request": a.RequestID(), "data.operator": a.Operator().ID, "data.specRevision": strconv.FormatInt(a.SpecificationRevision(), 10), "data.site": a.Site()} {
			envelope.ProductCandidate.Attributes[key] = value
		}
		store, err := sourcestore.NewTransactionWriter(tx, newCatalogBridge)
		if err != nil {
			return collection.Source{}, err
		}
		descriptor := sourcing.ProducerDescriptor{Kind: "custom_dataset", Version: "v1"}
		producer, err := sourcing.NewInternalProducer(declaredAuthority{a}, store, descriptor)
		if err != nil {
			return collection.Source{}, err
		}
		receipt, err := producer.Publish(ctx, sourcing.PublicationCommand{PublicationID: operation, Producer: descriptor, ProductKey: "custom-" + operation, Envelope: envelope})
		if err != nil {
			return collection.Source{}, err
		}
		return collection.Source{ProductKey: receipt.ProductKey, PublicationID: receipt.PublicationID, Version: receipt.CatalogVersion, OperationID: operation, Kind: "custom_dataset"}, nil
	}, nil
}
