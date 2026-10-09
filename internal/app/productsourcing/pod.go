package productsourcing

import (
	"context"
	"gorm.io/gorm"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	sourcingstore "task-processor/internal/integration/persistence/product/sourcing"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/sourcing"
	"time"
)

func ReceivePOD(ctx context.Context, tx *gorm.DB, authorization collection.Authorizer, scope collection.Scope, operation, kind, name string, envelope sourcing.SourceEnvelope) (collection.Receipt, error) {
	if tx == nil || authorization == nil || kind != "sds_template" && kind != "sds_finished" || envelope.Identity.SourcePlatform != "sds" || envelope.RawReference.ReferenceType != kind || envelope.Trace.SourceRunID != operation {
		return collection.Receipt{}, collection.ErrInvalid
	}
	if _, ok := tx.Statement.ConnPool.(gorm.TxCommitter); !ok {
		return collection.Receipt{}, collection.ErrUnavailable
	}
	store, e := sourcingstore.NewTransactionWriter(tx, newCatalogBridge)
	if e != nil {
		return collection.Receipt{}, e
	}
	producer := sourcing.ProducerDescriptor{Kind: kind, Version: "v1"}
	publisher, e := sourcing.NewInternalProducer(ownPublicationAuthority{auth: authorization, expected: scope}, store, producer)
	if e != nil {
		return collection.Receipt{}, e
	}
	zero := uint64(0)
	published, e := publisher.Publish(ctx, sourcing.PublicationCommand{PublicationID: operation, ProductKey: kind + "-" + operation, Producer: producer, ExpectedBaseVersion: &zero, Envelope: envelope})
	if e != nil {
		return collection.Receipt{}, e
	}
	return collectionstore.AppendReceived(ctx, tx, scope, operation, time.Now().Format("01/02 15:04")+" · "+name, collection.Source{ProductKey: published.ProductKey, PublicationID: published.PublicationID, Version: published.CatalogVersion, OperationID: operation, Kind: kind}, published.PublishedAt)
}
