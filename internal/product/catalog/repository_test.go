package catalog

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type recordingSnapshotWriter struct {
	published PublishRequest
	result    PublishedSnapshot
	err       error
}

func (w *recordingSnapshotWriter) PublishSnapshot(_ context.Context, request PublishRequest) (PublishedSnapshot, error) {
	w.published = request
	return w.result, w.err
}

func TestPublisherRejectsIncompleteExactIdentity(t *testing.T) {
	t.Parallel()

	writer := &recordingSnapshotWriter{}
	publisher, err := NewPublisher(writer)
	if err != nil {
		t.Fatalf("NewPublisher() error = %v", err)
	}

	for _, request := range []PublishRequest{
		{Identity: SnapshotIdentity{ProductKey: "product-1"}, PublicationID: "source-run-1", Snapshot: ProductSnapshot{Title: "Bottle"}},
		{Identity: SnapshotIdentity{TenantID: "tenant-a"}, PublicationID: "source-run-1", Snapshot: ProductSnapshot{Title: "Bottle"}},
		{Identity: SnapshotIdentity{TenantID: "tenant-a", ProductKey: "product-1"}, Snapshot: ProductSnapshot{Title: "Bottle"}},
		{Identity: SnapshotIdentity{TenantID: " tenant-a ", ProductKey: "product-1"}, PublicationID: "source-run-1", Snapshot: ProductSnapshot{Title: "Bottle"}},
		{Identity: SnapshotIdentity{TenantID: "tenant-a", ProductKey: " product-1 "}, PublicationID: "source-run-1", Snapshot: ProductSnapshot{Title: "Bottle"}},
		{Identity: SnapshotIdentity{TenantID: "tenant-a", ProductKey: "product-1"}, PublicationID: " source-run-1 ", Snapshot: ProductSnapshot{Title: "Bottle"}},
	} {
		if _, err := publisher.Publish(context.Background(), request); !errors.Is(err, ErrInvalidPublication) {
			t.Fatalf("Publish(%+v) error = %v, want ErrInvalidPublication", request, err)
		}
	}
	if writer.published.PublicationID != "" {
		t.Fatalf("writer received invalid publication: %+v", writer.published)
	}
}

func TestPublisherPassesNormalizedSnapshotToWriterWithoutAliasing(t *testing.T) {
	t.Parallel()

	writer := &recordingSnapshotWriter{result: PublishedSnapshot{
		Identity:      SnapshotIdentity{TenantID: "tenant-a", ProductKey: "product-1"},
		Version:       1,
		PublicationID: "source-run-1",
		Snapshot:      ProductSnapshot{Title: "Bottle", Attributes: []Attribute{{Name: "color", Value: "black"}}},
	}}
	publisher, err := NewPublisher(writer)
	if err != nil {
		t.Fatalf("NewPublisher() error = %v", err)
	}
	input := PublishRequest{
		Identity:      SnapshotIdentity{TenantID: "tenant-a", ProductKey: "product-1"},
		PublicationID: "source-run-1",
		Snapshot:      ProductSnapshot{Title: "Bottle", Attributes: []Attribute{{Name: "color", Value: "black"}}},
	}

	got, err := publisher.Publish(context.Background(), input)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if !reflect.DeepEqual(got, writer.result) {
		t.Fatalf("Publish() = %+v, want %+v", got, writer.result)
	}
	input.Snapshot.Attributes[0].Value = "mutated"
	if writer.published.Snapshot.Attributes[0].Value != "black" {
		t.Fatalf("writer snapshot aliased caller input: %+v", writer.published.Snapshot)
	}
	got.Snapshot.Attributes[0].Value = "mutated-result"
	if writer.result.Snapshot.Attributes[0].Value != "black" {
		t.Fatalf("publisher result aliased writer result: %+v", writer.result)
	}
}

func TestNewPublisherRejectsMissingWriter(t *testing.T) {
	t.Parallel()

	publisher, err := NewPublisher(nil)
	if publisher != nil || !errors.Is(err, ErrRepositoryUnavailable) {
		t.Fatalf("NewPublisher(nil) = (%T, %v), want nil ErrRepositoryUnavailable", publisher, err)
	}
}

func TestPublisherRejectsWriterResultWithDifferentIdentity(t *testing.T) {
	t.Parallel()

	writer := &recordingSnapshotWriter{result: PublishedSnapshot{
		Identity: SnapshotIdentity{TenantID: "tenant-b", ProductKey: "product-1"},
		Version:  1, PublicationID: "source-run-1", Snapshot: ProductSnapshot{Title: "Bottle"},
	}}
	publisher, err := NewPublisher(writer)
	if err != nil {
		t.Fatalf("NewPublisher() error = %v", err)
	}
	_, err = publisher.Publish(context.Background(), PublishRequest{
		Identity:      SnapshotIdentity{TenantID: "tenant-a", ProductKey: "product-1"},
		PublicationID: "source-run-1", Snapshot: ProductSnapshot{Title: "Bottle"},
	})
	if !errors.Is(err, ErrRepositoryStateInvalid) {
		t.Fatalf("Publish() error = %v, want ErrRepositoryStateInvalid", err)
	}
}

func TestPublisherRejectsWriterResultWithDifferentSnapshot(t *testing.T) {
	t.Parallel()

	identity := SnapshotIdentity{TenantID: "tenant-a", ProductKey: "product-1"}
	writer := &recordingSnapshotWriter{result: PublishedSnapshot{
		Identity: identity, Version: 1, PublicationID: "source-run-1",
		Snapshot: ProductSnapshot{Title: "different product facts"},
	}}
	publisher, err := NewPublisher(writer)
	if err != nil {
		t.Fatalf("NewPublisher() error = %v", err)
	}
	_, err = publisher.Publish(context.Background(), PublishRequest{
		Identity: identity, PublicationID: "source-run-1",
		Snapshot: ProductSnapshot{Title: "requested product facts"},
	})
	if !errors.Is(err, ErrRepositoryStateInvalid) {
		t.Fatalf("Publish() error = %v, want ErrRepositoryStateInvalid", err)
	}
}
