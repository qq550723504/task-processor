package httpapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/listingkit"
	"task-processor/internal/product/catalog"
)

func TestListingKitProductSnapshotReaderUsesPinnedVersion(t *testing.T) {
	reader := versionedCatalogSnapshotReader{snapshot: catalog.ProductSnapshot{Title: "Pinned Bottle"}}
	got, err := newListingKitProductSnapshotReader(&reader).GetProductSnapshot(context.Background(), listingkit.ProductSnapshotQuery{
		TenantID: "tenant-a", ProductKey: "product-1", Version: 7,
	})
	require.NoError(t, err)
	require.Equal(t, "Pinned Bottle", got.Title)
	require.Equal(t, uint64(7), reader.version)
	require.False(t, reader.currentCalled)
}

type versionedCatalogSnapshotReader struct {
	snapshot      catalog.ProductSnapshot
	version       uint64
	currentCalled bool
}

func (r *versionedCatalogSnapshotReader) GetCurrentSnapshot(context.Context, catalog.SnapshotIdentity) (catalog.PublishedSnapshot, error) {
	r.currentCalled = true
	return catalog.PublishedSnapshot{Version: 1, Snapshot: r.snapshot}, nil
}

func (r *versionedCatalogSnapshotReader) GetSnapshot(_ context.Context, _ catalog.SnapshotIdentity, version uint64) (catalog.PublishedSnapshot, error) {
	r.version = version
	return catalog.PublishedSnapshot{Version: version, Snapshot: r.snapshot}, nil
}
