package assetpersistence

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
	productasset "task-processor/internal/product/asset"
)

func TestBoundedExactApprovalReadIgnoresCurrentHeadAndRejectsCorruption(t *testing.T) {
	dsn := os.Getenv("ISSUE487_TEST_DSN")
	if dsn == "" {
		t.Skip("requires isolated PostgreSQL ISSUE487_TEST_DSN")
	}
	root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	schema := "approval487_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, root.Exec("CREATE SCHEMA "+schema).Error)
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		pool, _ := db.DB()
		_ = pool.Close()
		_ = root.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		rootPool, _ := root.DB()
		_ = rootPool.Close()
	})
	require.NoError(t, AutoMigrate(db))
	writer, err := NewRepository(db)
	require.NoError(t, err)
	first := repositoryTestCommit("org-1", "product-1", "approval-first", "asset-first")
	first.TargetPlatform, first.SourceSnapshotVersion = "product", 1
	_, err = writer.CommitApproval(context.Background(), first)
	require.NoError(t, err)
	second := repositoryTestCommit("org-1", "product-1", "approval-second", "asset-second")
	second.TargetPlatform, second.SourceSnapshotVersion = "product", 1
	_, err = writer.CommitApproval(context.Background(), second)
	require.NoError(t, err)
	reader, err := NewBoundedApprovalCommitReader(db, 2<<20)
	require.NoError(t, err)
	got, err := reader.ReadApprovalCommit(context.Background(), first.TenantID, first.ActionID)
	require.NoError(t, err)
	require.Equal(t, first, got, "exact immutable receipt must not follow the newer inventory head")
	_, err = reader.ReadApprovalCommit(context.Background(), "org-other", first.ActionID)
	require.ErrorIs(t, err, productasset.ErrApprovedAssetsNotReady)
	_, err = reader.ReadApprovalCommit(context.Background(), first.TenantID, "not-the-action")
	require.ErrorIs(t, err, productasset.ErrApprovedAssetsNotReady)
	tiny, err := NewBoundedApprovalCommitReader(db, 16)
	require.NoError(t, err)
	_, err = tiny.ReadApprovalCommit(context.Background(), first.TenantID, first.ActionID)
	require.ErrorIs(t, err, productasset.ErrInventoryTooLarge)
	require.NoError(t, db.Model(&ApprovedAssetRecord{}).Where("tenant_id = ? AND action_id = ?", first.TenantID, first.ActionID).Update("payload_json", []byte(`{"id":"corrupt"}`)).Error)
	_, err = reader.ReadApprovalCommit(context.Background(), first.TenantID, first.ActionID)
	require.ErrorIs(t, err, productasset.ErrRepositoryStateInvalid)
}
