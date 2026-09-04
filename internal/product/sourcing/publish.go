package sourcing

import (
	"context"
	"fmt"

	"task-processor/internal/product/catalog"
)

// PublishRequest carries only explicit publication identity and a structured
// source envelope. No identity is inferred from candidate text or legacy task
// results.
type PublishRequest struct {
	TenantID      string
	ProductKey    string
	PublicationID string
	Envelope      SourceEnvelope
}

type Publisher struct {
	catalog *catalog.Publisher
}

func NewPublisher(catalogPublisher *catalog.Publisher) (*Publisher, error) {
	if catalogPublisher == nil {
		return nil, fmt.Errorf("%w: catalog publisher is required", catalog.ErrRepositoryUnavailable)
	}
	return &Publisher{catalog: catalogPublisher}, nil
}

func (p *Publisher) Publish(ctx context.Context, request PublishRequest) (catalog.PublishedSnapshot, error) {
	if p == nil || p.catalog == nil {
		return catalog.PublishedSnapshot{}, fmt.Errorf("%w: catalog publisher is required", catalog.ErrRepositoryUnavailable)
	}
	snapshot, err := ToSnapshot(request.Envelope)
	if err != nil {
		return catalog.PublishedSnapshot{}, err
	}
	return p.catalog.Publish(ctx, catalog.PublishRequest{
		Identity:      catalog.SnapshotIdentity{TenantID: request.TenantID, ProductKey: request.ProductKey},
		PublicationID: request.PublicationID,
		Snapshot:      snapshot,
	})
}
