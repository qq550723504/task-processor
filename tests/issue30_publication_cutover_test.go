package tests

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	catalogdb "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/product/catalog"
)

func TestIssue30PublicationIdentityCutoverRequiresMigration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "cutover.db")
	db := issue30OpenDB(t, path)
	require.NoError(t, catalogdb.AutoMigrate(db))
	publisher, _ := issue30Publisher(t, db)
	envelope := issue30Envelope()
	envelope.Trace.SourceRunID = ""
	require.NotEmpty(t, envelope.RawReference.Checksum)
	next := issue30Request(t, envelope)
	old := next
	// Historical persisted ID, reproduced from sourcehandoff/a1688/command.go.
	// Evidence only: never use this formula in a new production importer.
	old.PublicationID = "source-snapshot:" + strings.TrimSpace(envelope.RawReference.Checksum)
	first, err := publisher.Publish(ctx, old)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	publisher, repo := issue30Publisher(t, issue30OpenDB(t, path))
	oldReplay, err := publisher.Publish(ctx, old)
	require.NoError(t, err)
	require.Equal(t, first, oldReplay)
	afterCutover, err := publisher.Publish(ctx, next)
	require.NoError(t, err)
	// Characterization of an OPEN rollout blocker, not a migration fix.
	// The red assertion first.Version == afterCutover.Version reproduced 1 != 2.
	require.NotEqual(t, old.PublicationID, next.PublicationID)
	require.Equal(t, first.Snapshot, afterCutover.Snapshot)
	require.Equal(t, first.Version+1, afterCutover.Version, "known unsafe cutover appends the identical snapshot")
	replay, err := publisher.Publish(ctx, next)
	require.NoError(t, err)
	require.Equal(t, afterCutover, replay)
	history, err := repo.(catalog.VersionedSnapshotReader).GetSnapshot(ctx, first.Identity, first.Version)
	require.NoError(t, err)
	require.Equal(t, first, history)
}
