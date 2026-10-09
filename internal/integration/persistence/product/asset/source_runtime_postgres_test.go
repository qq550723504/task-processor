//go:build integration

package assetpersistence

import (
	"context"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	"testing"
	"time"
)

func TestSourceRuntimeUsesExistingAssetOwnerWithNarrowPermissions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	owner, _ := openApprovedAssetPostgres(t, ctx)
	require.NoError(t, InstallSourceApprovalSchema(owner))
	require.NoError(t, VerifySourceApprovalSchema(ctx, owner))
	require.NoError(t, owner.Exec(`CREATE ROLE supply_asset_runtime LOGIN PASSWORD 'isolated_asset' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS`).Error)
	require.NoError(t, GrantSourceRuntimePermissions(ctx, owner))
	dsn, err := url.Parse(owner.Dialector.(*postgres.Dialector).Config.DSN)
	require.NoError(t, err)
	dsn.User = url.UserPassword(SourceRuntimeRole, "isolated_asset")
	runtimeDB, err := gorm.Open(postgres.Open(dsn.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := runtimeDB.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(2)
	t.Cleanup(func() { _ = pool.Close() })
	require.NoError(t, VerifySourceRuntimePermissions(ctx, runtimeDB))
	repository, err := NewRepository(runtimeDB)
	require.NoError(t, err)
	_, err = repository.CommitApproval(ctx, repositoryTestCommit("org", "product", "approve", "asset"))
	require.NoError(t, err)
	require.NoError(t, owner.Exec(`GRANT UPDATE(payload_json) ON product_approved_assets TO supply_asset_runtime`).Error)
	require.Error(t, VerifySourceRuntimePermissions(ctx, runtimeDB), "immutable assets cannot gain column writes")
	require.NoError(t, owner.Exec(`REVOKE UPDATE(payload_json) ON product_approved_assets FROM supply_asset_runtime`).Error)
	require.NoError(t, VerifySourceRuntimePermissions(ctx, runtimeDB))
	require.NoError(t, owner.Exec(`CREATE TABLE foreign_fact(id text)`).Error)
	require.NoError(t, owner.Exec(`GRANT SELECT ON foreign_fact TO supply_asset_runtime`).Error)
	require.Error(t, VerifySourceRuntimePermissions(ctx, runtimeDB), "foreign owner access must close module admission")
}
func TestSourceSchemaRejectsSameNamedUniqueIndexWithWrongKeys(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	owner, _ := openApprovedAssetPostgres(t, ctx)
	require.NoError(t, InstallSourceApprovalSchema(owner))
	require.NoError(t, VerifySourceApprovalSchema(ctx, owner))
	require.NoError(t, owner.Exec(`DROP INDEX ux_product_approval_origin`).Error)
	require.NoError(t, owner.Exec(`CREATE UNIQUE INDEX ux_product_approval_origin ON product_approved_assets(tenant_id)`).Error)
	require.Error(t, VerifySourceApprovalSchema(ctx, owner), "origin deduplication needs exact tenant action kind identity keys")
}
