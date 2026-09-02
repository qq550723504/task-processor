package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Publisher owns validation and defensive copying at the Catalog write
// boundary. Persistence adapters own the atomic version/idempotency commit.
type Publisher struct {
	writer SnapshotWriter
}

func NewPublisher(writer SnapshotWriter) (*Publisher, error) {
	if writer == nil {
		return nil, fmt.Errorf("%w: snapshot writer is required", ErrRepositoryUnavailable)
	}
	return &Publisher{writer: writer}, nil
}

func (p *Publisher) Publish(ctx context.Context, request PublishRequest) (PublishedSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return PublishedSnapshot{}, err
	}
	if p == nil || p.writer == nil {
		return PublishedSnapshot{}, fmt.Errorf("%w: snapshot writer is required", ErrRepositoryUnavailable)
	}
	if err := ValidatePublishRequest(request); err != nil {
		return PublishedSnapshot{}, err
	}
	cloned, err := CloneProductSnapshot(request.Snapshot)
	if err != nil {
		return PublishedSnapshot{}, err
	}
	request.Snapshot = cloned
	published, err := p.writer.PublishSnapshot(ctx, request)
	if err != nil {
		return PublishedSnapshot{}, err
	}
	if published.Identity != request.Identity || published.PublicationID != request.PublicationID || published.Version == 0 {
		return PublishedSnapshot{}, fmt.Errorf("%w: writer returned mismatched publication identity", ErrRepositoryStateInvalid)
	}
	published.Snapshot, err = CloneProductSnapshot(published.Snapshot)
	if err != nil {
		return PublishedSnapshot{}, err
	}
	return published, nil
}

func ValidatePublishRequest(request PublishRequest) error {
	if strings.TrimSpace(request.Identity.TenantID) == "" ||
		strings.TrimSpace(request.Identity.ProductKey) == "" ||
		strings.TrimSpace(request.PublicationID) == "" ||
		request.Identity.TenantID != strings.TrimSpace(request.Identity.TenantID) ||
		request.Identity.ProductKey != strings.TrimSpace(request.Identity.ProductKey) ||
		request.PublicationID != strings.TrimSpace(request.PublicationID) {
		return ErrInvalidPublication
	}
	if _, err := CloneProductSnapshot(request.Snapshot); err != nil {
		return err
	}
	return nil
}

func ValidateSnapshotIdentity(identity SnapshotIdentity) error {
	if strings.TrimSpace(identity.TenantID) == "" || strings.TrimSpace(identity.ProductKey) == "" ||
		identity.TenantID != strings.TrimSpace(identity.TenantID) || identity.ProductKey != strings.TrimSpace(identity.ProductKey) {
		return ErrInvalidPublication
	}
	return nil
}

func CloneProductSnapshot(snapshot ProductSnapshot) (ProductSnapshot, error) {
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return ProductSnapshot{}, fmt.Errorf("%w: encode snapshot: %v", ErrInvalidSnapshot, err)
	}
	if string(encoded) == "{}" {
		return ProductSnapshot{}, ErrInvalidSnapshot
	}
	var cloned ProductSnapshot
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		return ProductSnapshot{}, fmt.Errorf("%w: decode snapshot: %v", ErrInvalidSnapshot, err)
	}
	return cloned, nil
}
