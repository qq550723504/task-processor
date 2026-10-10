//go:build integration

package sheinobservations

import (
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"net/url"
	"task-processor/internal/storecenter"
	"testing"
)

func TestStoreObservationEnabledRuntimeHasExactSharedGrants(t *testing.T) {
	ctx, owner, _ := observationDatabase(t)
	pool, err := owner.DB()
	require.NoError(t, err)
	tx, err := pool.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, storecenter.InstallCurrentSchemaTx(ctx, tx))
	require.NoError(t, tx.Commit())
	for _, q := range []string{
		`CREATE ROLE store_center_runtime LOGIN PASSWORD 'synthetic-observation-test'`,
		`GRANT CONNECT ON DATABASE store_observations_test TO store_center_runtime`,
		`GRANT USAGE ON SCHEMA public TO store_center_runtime`,
		`GRANT SELECT,INSERT,UPDATE ON workbench_stores,workbench_store_member_grants,workbench_store_service_operations,workbench_store_connections,workbench_store_connection_attempts TO store_center_runtime`,
		`GRANT SELECT,INSERT ON workbench_store_audit_logs,workbench_store_member_grant_operations,workbench_store_merchant_bindings TO store_center_runtime`,
	} {
		require.NoError(t, owner.Exec(q).Error)
	}
	u, err := url.Parse(owner.Dialector.(*postgres.Dialector).Config.DSN)
	require.NoError(t, err)
	u.User = url.UserPassword("store_center_runtime", "synthetic-observation-test")
	serving, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{})
	require.NoError(t, err)
	servingPool, err := serving.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = servingPool.Close() })
	require.NoError(t, storecenter.VerifyRuntimePermissions(ctx, serving))
	enabled := storecenter.RuntimeCapabilities{Observations: true}
	require.Error(t, storecenter.VerifyRuntimePermissionsForCapabilities(ctx, serving, enabled), "enabled requires every observation grant")
	require.NoError(t, GrantRuntime(ctx, owner, "store_center_runtime"))
	require.Error(t, storecenter.VerifyRuntimePermissions(ctx, serving), "disabled remains strict")
	require.NoError(t, storecenter.VerifyRuntimePermissionsForCapabilities(ctx, serving, enabled))
	require.NoError(t, storecenter.VerifyCurrentSchema(ctx, serving))
	_, err = NewRepository(ctx, serving)
	require.NoError(t, err)
	for _, q := range []string{
		`GRANT DELETE ON shein_observations.records TO store_center_runtime`,
		`GRANT UPDATE ON shein_observations.commands TO store_center_runtime`,
		`GRANT CREATE ON SCHEMA shein_observations TO store_center_runtime`,
		`REVOKE UPDATE ON shein_observations.heads FROM store_center_runtime`,
	} {
		require.NoError(t, owner.Exec(q).Error)
		require.Error(t, storecenter.VerifyRuntimePermissionsForCapabilities(ctx, serving, enabled), q)
		require.NoError(t, owner.Exec(`REVOKE DELETE ON shein_observations.records FROM store_center_runtime; REVOKE UPDATE ON shein_observations.commands FROM store_center_runtime; REVOKE CREATE ON SCHEMA shein_observations FROM store_center_runtime`).Error)
		require.NoError(t, GrantRuntime(ctx, owner, "store_center_runtime"))
	}
	require.NoError(t, owner.Exec(`CREATE TABLE public.unrelated_private(value text); GRANT SELECT(value) ON public.unrelated_private TO store_center_runtime`).Error)
	require.Error(t, storecenter.VerifyRuntimePermissionsForCapabilities(ctx, serving, enabled), "enabled must not allow unrelated column grants")
}
