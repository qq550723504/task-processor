package dataservicesapp

import (
	"context"
	"encoding/json"
	"gorm.io/gorm"
	sourcestore "task-processor/internal/integration/persistence/product/sourcing"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/dataacquisition"
	"task-processor/internal/product/sourcing"
)

type capturedResultReader struct{ store sourcing.PublicationReadStore }

func transactionResultReader(tx *gorm.DB) (dataacquisition.CapturedResultReader, error) {
	reader, err := sourcestore.NewTransactionReader(tx, newCatalogBridge)
	if err != nil {
		return nil, err
	}
	return capturedResultReader{reader}, nil
}

func (r capturedResultReader) Verify(ctx context.Context, scope collection.Scope, source collection.Source, evidence dataacquisition.Evidence) error {
	if scope.Validate() != nil || source.Kind != "amazon_data" || source.PublicationID != source.OperationID || !collection.ValidID(source.PublicationID) || evidence.Validate() != nil {
		return dataacquisition.ErrInvalid
	}
	p, err := r.store.Read(ctx, scope.OrganizationID, source.PublicationID)
	if err != nil {
		return dataacquisition.ErrUnavailable
	}
	raw, err := json.Marshal(evidence)
	if err != nil {
		return dataacquisition.ErrUnavailable
	}
	if p.Receipt.ActorID != scope.ActorID || p.Receipt.OrganizationID != scope.OrganizationID || p.Receipt.ProductKey != source.ProductKey || p.Receipt.PublicationID != source.PublicationID || p.Receipt.CatalogVersion != source.Version || p.Receipt.Producer.Kind != "amazon_data" || p.Envelope.RawReference.Checksum != sourcing.RawSnapshotChecksum(string(raw)) {
		return dataacquisition.ErrUnavailable
	}
	return nil
}
