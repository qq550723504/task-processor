package review

import (
	"context"
	"encoding/json"
	"reflect"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

// Binding is injected by the admitted upstream setup, never decoded from HTTP.
// Exact publications, not all rows in a tenant, form this application's input set.
type Binding struct {
	Identity      catalog.SnapshotIdentity
	Version       uint64
	PublicationID string
	Source        sourcing.SourceEnvelope
}

func (s *Service) source(ctx context.Context, reader catalog.VersionedSnapshotReader, org string, in CreateInput) (catalog.PublishedSnapshot, sourcing.SourceEnvelope, error) {
	key := bindingKey{org, in.ProductKey, in.BaseVersion}
	b, ok := s.bindings[key]
	if !ok {
		return catalog.PublishedSnapshot{}, sourcing.SourceEnvelope{}, ErrNotFound
	}
	p, err := reader.GetSnapshot(ctx, b.Identity, b.Version)
	if err != nil {
		return p, b.Source, err
	}
	expected, err := sourcing.ToSnapshot(b.Source)
	if err != nil || p.Identity != b.Identity || p.Version != b.Version || p.PublicationID != b.PublicationID || !reflect.DeepEqual(p.Snapshot.Sources, expected.Sources) {
		return p, b.Source, ErrNotFound
	}
	return p, b.Source, nil
}

type bindingKey struct {
	org, key string
	version  uint64
}

func cloneBindings(bindings []Binding) (map[bindingKey]Binding, error) {
	raw, err := json.Marshal(bindings)
	if err != nil {
		return nil, ErrInvalid
	}
	var copyBindings []Binding
	if json.Unmarshal(raw, &copyBindings) != nil {
		return nil, ErrInvalid
	}
	result := map[bindingKey]Binding{}
	for _, b := range copyBindings {
		if catalog.ValidateSnapshotIdentity(b.Identity) != nil || b.Version == 0 || b.Version > 1<<63-1 || !ValidKey(b.PublicationID) || !b.Source.Identity.Valid() {
			return nil, ErrInvalid
		}
		key := bindingKey{b.Identity.TenantID, b.Identity.ProductKey, b.Version}
		if _, exists := result[key]; exists {
			return nil, ErrInvalid
		}
		result[key] = b
	}
	return result, nil
}
