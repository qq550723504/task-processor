package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestPublisherBoundsEncodedSnapshotBeforeWriter(t *testing.T) {
	writer := &sizeRecordingWriter{}
	publisher, err := NewPublisher(writer)
	if err != nil {
		t.Fatalf("NewPublisher(): %v", err)
	}
	request := PublishRequest{
		Identity:      SnapshotIdentity{TenantID: "tenant-a", ProductKey: "product-a"},
		PublicationID: "publication-1",
		Snapshot:      snapshotWithEncodedSize(t, MaxEncodedSnapshotBytes),
	}

	if _, err := publisher.Publish(context.Background(), request); err != nil {
		t.Fatalf("Publish(exact limit): %v", err)
	}
	if writer.calls != 1 {
		t.Fatalf("writer calls = %d, want 1", writer.calls)
	}

	request.PublicationID = "publication-2"
	request.Snapshot = snapshotWithEncodedSize(t, MaxEncodedSnapshotBytes+1)
	if _, err := publisher.Publish(context.Background(), request); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("Publish(over limit) error = %v, want ErrInvalidSnapshot", err)
	}
	if writer.calls != 1 {
		t.Fatalf("writer called for oversized snapshot: %d", writer.calls)
	}
}

type sizeRecordingWriter struct{ calls int }

func (w *sizeRecordingWriter) PublishSnapshot(_ context.Context, request PublishRequest) (PublishedSnapshot, error) {
	w.calls++
	return PublishedSnapshot{Identity: request.Identity, PublicationID: request.PublicationID, Version: 1, Snapshot: request.Snapshot}, nil
}

func snapshotWithEncodedSize(t *testing.T, size int) ProductSnapshot {
	t.Helper()
	probe, err := json.Marshal(ProductSnapshot{Description: "x"})
	if err != nil {
		t.Fatalf("marshal probe: %v", err)
	}
	overhead := len(probe) - 1
	if size <= overhead {
		t.Fatalf("requested encoded size %d is too small", size)
	}
	snapshot := ProductSnapshot{Description: strings.Repeat("x", size-overhead)}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if len(encoded) != size {
		t.Fatalf("encoded size = %d, want %d", len(encoded), size)
	}
	return snapshot
}
