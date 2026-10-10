package operationscockpitpersistence

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	"task-processor/internal/storecenter"
	"testing"
	"time"
)

type liveStoreFixture struct{ admin bool }

func (a liveStoreFixture) AuthorizeStoreMember(_ context.Context, org string) (storecenter.StoreMemberAccess, error) {
	return storecenter.StoreMemberAccess{OrganizationID: org, ActorID: "actor-a", MemberID: "member-a", Administrator: a.admin}, nil
}

func TestPostgresCockpitSharedCapabilityAndBorrowedStoreLocks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("cockpit_shared"), tcpostgres.WithUsername("fixture_owner"), tcpostgres.WithPassword("isolated-cockpit-test"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	owner, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := owner.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = pool.Close() })
	tx, err := pool.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, storecenter.InstallCurrentSchemaTx(ctx, tx))
	require.NoError(t, tx.Commit())
	require.NoError(t, Install(ctx, owner))
	for _, sql := range []string{
		`CREATE ROLE store_center_runtime LOGIN PASSWORD 'isolated-shared-runtime'`,
		`REVOKE CREATE,TEMP ON DATABASE cockpit_shared FROM PUBLIC`,
		`GRANT CONNECT ON DATABASE cockpit_shared TO store_center_runtime`,
		`GRANT USAGE ON SCHEMA public TO store_center_runtime`,
		`GRANT SELECT,INSERT,UPDATE ON workbench_stores,workbench_store_member_grants,workbench_store_service_operations,workbench_store_connections,workbench_store_connection_attempts TO store_center_runtime`,
		`GRANT SELECT,INSERT ON workbench_store_audit_logs,workbench_store_member_grant_operations,workbench_store_merchant_bindings TO store_center_runtime`,
	} {
		require.NoError(t, owner.Exec(sql).Error)
	}
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.User = url.UserPassword("store_center_runtime", "isolated-shared-runtime")
	runtimeDB, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	runtimePool, err := runtimeDB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = runtimePool.Close() })
	enabled := storecenter.RuntimeCapabilities{OperationsCockpit: true}
	require.NoError(t, storecenter.VerifyRuntimePermissions(ctx, runtimeDB))
	require.Error(t, storecenter.VerifyRuntimePermissionsForCapabilities(ctx, runtimeDB, enabled), "flag requires all grants")
	require.NoError(t, GrantRuntime(ctx, owner, "store_center_runtime"))
	require.Error(t, storecenter.VerifyRuntimePermissions(ctx, runtimeDB), "disabled rejects feature grants")
	require.NoError(t, storecenter.VerifyRuntimePermissionsForCapabilities(ctx, runtimeDB, enabled))
	for _, change := range []struct{ add, undo string }{
		{`GRANT UPDATE ON operations_cockpit.goal_versions TO store_center_runtime`, `REVOKE UPDATE ON operations_cockpit.goal_versions FROM store_center_runtime`},
		{`GRANT DELETE ON operations_cockpit.facts TO store_center_runtime`, `REVOKE DELETE ON operations_cockpit.facts FROM store_center_runtime`},
		{`REVOKE INSERT ON operations_cockpit.commands FROM store_center_runtime`, `GRANT INSERT ON operations_cockpit.commands TO store_center_runtime`},
	} {
		require.NoError(t, owner.Exec(change.add).Error)
		require.Error(t, storecenter.VerifyRuntimePermissionsForCapabilities(ctx, runtimeDB, enabled))
		require.NoError(t, owner.Exec(change.undo).Error)
	}
	id := uuid.NewString()
	require.NoError(t, owner.Exec(`INSERT INTO workbench_stores(id,organization_id,name,platform,region,external_store_id,record_status,service_status,connection_ref,version,created_by,updated_by,created_at,updated_at,create_idempotency_key,delete_operation_key,identity_key,create_request_fingerprint) VALUES(?,'org-a','Fixture','shein','SG','fixture','active','pending_activation','fixture',1,'actor-a','actor-a',now(),now(),?,'','fixture','fixture')`, id, uuid.NewString()).Error)
	require.NoError(t, owner.Exec(`INSERT INTO workbench_store_member_grants VALUES('org-a',?,'member-a',true,1,'actor-a',now())`, id).Error)
	for _, admin := range []bool{false, true} {
		borrowed := runtimeDB.WithContext(ctx).Begin()
		require.NoError(t, borrowed.Error)
		repo, err := storecenter.NewMemberScopedStoreRepository(borrowed, liveStoreFixture{admin})
		require.NoError(t, err)
		require.NoError(t, repo.LockRead(ctx, "org-a", []string{id}))
		// A competing owner transaction cannot acquire the exact native Store/grant
		// rows until the borrowed caller completes, including the administrator path.
		competitor := owner.WithContext(ctx).Begin()
		require.NoError(t, competitor.Error)
		require.NoError(t, competitor.Exec(`SET LOCAL lock_timeout='50ms'`).Error)
		require.Error(t, competitor.Exec(`UPDATE workbench_stores SET version=version+1 WHERE id=?`, id).Error)
		require.NoError(t, competitor.Rollback().Error)
		if !admin {
			competitor = owner.WithContext(ctx).Begin()
			require.NoError(t, competitor.Exec(`SET LOCAL lock_timeout='50ms'`).Error)
			require.Error(t, competitor.Exec(`UPDATE workbench_store_member_grants SET active=false WHERE store_id=?`, id).Error)
			require.NoError(t, competitor.Rollback().Error)
		}
		require.NoError(t, borrowed.Commit().Error)
		require.NoError(t, owner.Exec(`UPDATE workbench_stores SET version=version+1 WHERE id=?`, id).Error)
	}
}
