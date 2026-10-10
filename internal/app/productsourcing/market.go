package productsourcing

import (
	"context"
	"gorm.io/gorm"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	sourcingstore "task-processor/internal/integration/persistence/product/sourcing"
	marketstore "task-processor/internal/integration/persistence/product/supplymarket"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/product/supplymarket"
	"time"
)

type marketReceiver struct {
	tx            *gorm.DB
	authorization collection.Authorizer
}

func NewMarketReceiver(tx *gorm.DB, authorization collection.Authorizer) (marketstore.Receiver, error) {
	if tx == nil || authorization == nil {
		return nil, supplymarket.ErrUnavailable
	}
	if _, ok := tx.Statement.ConnPool.(gorm.TxCommitter); !ok {
		return nil, supplymarket.ErrUnavailable
	}
	return marketReceiver{tx, authorization}, nil
}
func (r marketReceiver) Receive(ctx context.Context, scope collection.Scope, operation string, release supplymarket.Release) (collection.Receipt, error) {
	envelope, err := supplymarket.ReleasedEnvelope(scope, operation, release)
	if err != nil {
		return collection.Receipt{}, err
	}
	store, err := sourcingstore.NewTransactionWriter(r.tx, newCatalogBridge)
	if err != nil {
		return collection.Receipt{}, err
	}
	producer := sourcing.ProducerDescriptor{Kind: "supply_market", Version: "v1"}
	publisher, err := sourcing.NewInternalProducer(ownPublicationAuthority{auth: r.authorization, expected: scope}, store, producer)
	if err != nil {
		return collection.Receipt{}, err
	}
	zero := uint64(0)
	published, err := publisher.Publish(ctx, sourcing.PublicationCommand{PublicationID: operation, ProductKey: "market-" + operation, Producer: producer, ExpectedBaseVersion: &zero, Envelope: envelope})
	if err != nil {
		return collection.Receipt{}, err
	}
	label := "硕米自营"
	if release.Channel == "selected" {
		label = "硕米优选"
	}
	return collectionstore.AppendReceived(ctx, r.tx, scope, operation, time.Now().Format("01/02 15:04")+" · "+label, collection.Source{ProductKey: published.ProductKey, PublicationID: published.PublicationID, Version: published.CatalogVersion, OperationID: operation, Kind: "market"}, published.PublishedAt)
}
