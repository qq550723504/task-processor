package catalog

import "context"

// SnapshotIdentity is the exact tenant-qualified identity of one canonical
// product. Callers must provide both fields; Catalog never derives either one.
type SnapshotIdentity struct {
	TenantID   string
	ProductKey string
}

// PublishRequest identifies one immutable publication attempt. PublicationID
// is the idempotency identity within the tenant-qualified product stream.
type PublishRequest struct {
	Identity      SnapshotIdentity
	PublicationID string
	Snapshot      ProductSnapshot
}

// PublishedSnapshot is one immutable version in a product snapshot stream.
type PublishedSnapshot struct {
	Identity      SnapshotIdentity
	Version       uint64
	PublicationID string
	Snapshot      ProductSnapshot
}

// SnapshotWriter is the Catalog-owned atomic publication port.
type SnapshotWriter interface {
	PublishSnapshot(context.Context, PublishRequest) (PublishedSnapshot, error)
}

// SnapshotReader is the narrow read-only port used by product consumers.
type SnapshotReader interface {
	GetCurrentSnapshot(context.Context, SnapshotIdentity) (PublishedSnapshot, error)
}

// VersionedSnapshotReader reads an immutable historical snapshot selected by
// its Catalog-assigned version. It is optional so legacy readers can continue
// serving current snapshots while deployments migrate.
type VersionedSnapshotReader interface {
	GetSnapshot(context.Context, SnapshotIdentity, uint64) (PublishedSnapshot, error)
}

// CompleteSnapshotReader exposes current and immutable historical reads
// without granting the Catalog publication capability.
type CompleteSnapshotReader interface {
	SnapshotReader
	VersionedSnapshotReader
}

// Repository combines Catalog's write and read ports for persistence adapters.
type Repository interface {
	SnapshotWriter
	SnapshotReader
}
