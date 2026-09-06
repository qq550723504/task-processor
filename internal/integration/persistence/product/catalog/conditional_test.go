package catalogpersistence

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/product/catalog"
)

func TestConditionalPublisherPostgres(t *testing.T) {
	dsn := os.Getenv("ISSUE333_TEST_DSN")
	if dsn == "" {
		t.Skip("requires isolated ISSUE333_TEST_DSN")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	schema := "catalog333_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, db.Exec("CREATE SCHEMA "+schema).Error)
	isolated, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		raw, _ := isolated.DB()
		_ = raw.Close()
		_ = db.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		raw, _ = db.DB()
		_ = raw.Close()
	})
	require.NoError(t, AutoMigrate(isolated))
	repo, err := NewRepository(isolated)
	require.NoError(t, err)
	publisher, err := catalog.NewPublisher(repo)
	require.NoError(t, err)
	ctx := context.Background()
	request := catalog.PublishRequest{Identity: catalog.SnapshotIdentity{TenantID: "org", ProductKey: "product"}, PublicationID: "initial", Snapshot: catalog.ProductSnapshot{Title: "initial"}}
	first, err := publisher.Publish(ctx, request)
	require.NoError(t, err)
	request.PublicationID = "conditional"
	request.Snapshot.Title = "accepted"
	request.ExpectedBaseVersion = &first.Version
	second, err := publisher.Publish(ctx, request)
	require.NoError(t, err)
	require.Equal(t, uint64(2), second.Version)
	replay, err := publisher.Publish(ctx, request)
	require.NoError(t, err)
	require.Equal(t, second, replay)
	request.PublicationID = "stale"
	_, err = publisher.Publish(ctx, request)
	require.ErrorIs(t, err, catalog.ErrStaleSnapshot)
	// Caller transaction controls commit: no committed version survives a failed receipt.
	request.ExpectedBaseVersion = &second.Version
	request.PublicationID = "rollback"
	err = isolated.Transaction(func(tx *gorm.DB) error {
		writer, e := NewTransactionWriter(tx)
		require.NoError(t, e)
		p, e := catalog.NewPublisher(writer)
		require.NoError(t, e)
		_, e = p.Publish(ctx, request)
		require.NoError(t, e)
		return context.Canceled
	})
	require.ErrorIs(t, err, context.Canceled)
	current, err := repo.GetCurrentSnapshot(ctx, request.Identity)
	require.NoError(t, err)
	require.Equal(t, second, current)
}
