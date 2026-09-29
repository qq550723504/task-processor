//go:build integration

package storecenter_test

import (
	"context"
	"errors"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/storecenter"
)

func TestNativeRecordPostgresConcurrencyRollbackAndDeleteReceipt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("native_records"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("synthetic-native-test"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
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
	require.NoError(t, storecenter.VerifyCurrentSchema(ctx, owner))
	for _, statement := range []string{
		`CREATE ROLE store_center_runtime LOGIN PASSWORD 'synthetic-native-test'`,
		`REVOKE CREATE ON SCHEMA public FROM PUBLIC`,
		`REVOKE ALL ON DATABASE native_records FROM PUBLIC`,
		`GRANT CONNECT ON DATABASE native_records TO store_center_runtime`,
		`GRANT USAGE ON SCHEMA public TO store_center_runtime`,
		`GRANT SELECT,INSERT,UPDATE ON workbench_stores,workbench_store_member_grants,workbench_store_service_operations,workbench_store_connections,workbench_store_connection_attempts TO store_center_runtime`,
		`GRANT SELECT,INSERT ON workbench_store_audit_logs,workbench_store_member_grant_operations,workbench_store_merchant_bindings TO store_center_runtime`,
	} {
		require.NoError(t, owner.Exec(statement).Error)
	}
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.User = url.UserPassword("store_center_runtime", "synthetic-native-test")
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	runtimePool, err := db.DB()
	require.NoError(t, err)
	runtimePool.SetMaxOpenConns(8)
	t.Cleanup(func() { _ = runtimePool.Close() })
	require.NoError(t, storecenter.VerifyRuntimePermissions(ctx, db))
	repo, err := storecenter.NewMemberScopedStoreRepository(db, &storeMemberAuthorizer{member: "membership-a"})
	require.NoError(t, err)
	key := uuid.NewString()
	candidates := make([]*storecenter.Store, 8)
	for i := range candidates {
		candidates[i] = nativeCandidate(t, key, "subject-create")
	}
	var group sync.WaitGroup
	results := make(chan *storecenter.Store, 8)
	failures := make(chan error, 8)
	for _, candidate := range candidates {
		group.Add(1)
		go func(candidate *storecenter.Store) {
			defer group.Done()
			stored, _, err := repo.CreateOrReplay(ctx, "org-a", candidate)
			if err != nil {
				failures <- err
			} else {
				results <- stored
			}
		}(candidate)
	}
	group.Wait()
	close(results)
	close(failures)
	for err := range failures {
		require.NoError(t, err)
	}
	var original *storecenter.Store
	for result := range results {
		if original == nil {
			original = result
		}
		require.Equal(t, original.ID(), result.ID())
	}
	require.NotNil(t, original)
	for _, table := range []string{"workbench_stores", "workbench_store_member_grants", "workbench_store_member_grant_operations", "workbench_store_audit_logs"} {
		var count int64
		require.NoError(t, owner.Table(table).Count(&count).Error)
		require.EqualValues(t, 1, count, table)
	}
	// A failed audit and a failed grant each roll back the Store and every receipt.
	for _, table := range []string{"workbench_store_audit_logs", "workbench_store_member_grants"} {
		require.NoError(t, owner.Exec(`CREATE FUNCTION fail_native_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic failure'; END $$`).Error)
		require.NoError(t, owner.Exec("CREATE TRIGGER fail_native BEFORE INSERT ON "+table+" FOR EACH ROW EXECUTE FUNCTION fail_native_write()").Error)
		candidate := nativeCandidate(t, uuid.NewString(), "subject-create")
		snapshot := candidate.Snapshot()
		snapshot.ExternalStoreID = uuid.NewString()
		candidate, err = storecenter.RehydrateStore(snapshot)
		require.NoError(t, err)
		_, _, err = repo.CreateOrReplay(ctx, "org-a", candidate)
		require.Error(t, err)
		var count int64
		require.NoError(t, owner.Table("workbench_stores").Count(&count).Error)
		require.EqualValues(t, 1, count)
		require.NoError(t, owner.Exec("DROP TRIGGER fail_native ON "+table).Error)
		require.NoError(t, owner.Exec("DROP FUNCTION fail_native_write()").Error)
	}
	// Delete is one native transaction. Its original version/key are immutable.
	admin, err := storecenter.NewMemberScopedStoreRepository(db, &storeMemberAuthorizer{member: "membership-admin", admin: true})
	require.NoError(t, err)
	request := storecenter.DeleteStoreRequest{OrganizationID: "org-a", ActorSubject: "subject-create", StoreID: original.ID(), OperationKey: uuid.NewString(), ExpectedVersion: original.Version()}
	deleted, err := admin.DeleteRecord(ctx, request, time.Now().UTC())
	require.NoError(t, err)
	replay, err := admin.DeleteRecord(ctx, request, time.Now().UTC())
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.Equal(t, deleted.Version, replay.Version)
	request.ExpectedVersion++
	_, err = admin.DeleteRecord(ctx, request, time.Now().UTC())
	require.ErrorIs(t, err, storecenter.ErrInvalidTransition)
	_, _, err = admin.CreateOrReplay(ctx, "org-a", nativeCandidate(t, key, "subject-create"))
	require.True(t, errors.Is(err, storecenter.ErrAlreadyExists))
	var active int64
	require.NoError(t, owner.Table("workbench_store_member_grants").Where("active=?", true).Count(&active).Error)
	require.Zero(t, active)
	// Service application and deletion lock the same native Store. Whichever
	// wins, the original paid operation remains readable and cannot be revived.
	for attempt := 0; attempt < 4; attempt++ {
		candidate := nativeCandidate(t, uuid.NewString(), "subject-create")
		snapshot := candidate.Snapshot()
		snapshot.ExternalStoreID = uuid.NewString()
		candidate, err = storecenter.RehydrateStore(snapshot)
		require.NoError(t, err)
		created, _, err := repo.CreateOrReplay(ctx, "org-a", candidate)
		require.NoError(t, err)
		intent := storecenter.ServiceChargeIntent{MemberID: "membership-a", Funding: "member_allocated", Execution: storecenter.ServiceExecution{OrganizationID: "org-a", StoreID: created.ID(), OperationID: uuid.NewString(), Command: storecenter.ServiceCommandActivate, Quantity: 1, MaxQuantity: 1, ExpectedStoreVersion: created.Version(), ActorSubject: "subject-create", OccurredAt: time.Now().UTC(), RequestFingerprint: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ConnectionStatus: storecenter.ConnectionStatusConnected}}
		_, err = repo.AdmitServiceCharge(ctx, intent)
		require.NoError(t, err)
		reservationID := uuid.NewString()
		require.NoError(t, repo.BindServiceCharge(ctx, intent, reservationID))
		deleteRequest := storecenter.DeleteStoreRequest{OrganizationID: "org-a", ActorSubject: "subject-create", StoreID: created.ID(), OperationKey: uuid.NewString(), ExpectedVersion: created.Version()}
		start := make(chan struct{})
		var proof storecenter.ServiceChargeProof
		var applyErr, deleteErr error
		group.Add(2)
		go func() {
			defer group.Done()
			<-start
			proof, applyErr = repo.ApplyServiceCharge(ctx, intent, storecenter.ConnectionStatusConnected)
		}()
		go func() {
			defer group.Done()
			<-start
			_, deleteErr = admin.DeleteRecord(ctx, deleteRequest, time.Now().UTC())
		}()
		close(start)
		group.Wait()
		require.NoError(t, applyErr)
		require.Equal(t, reservationID, proof.ReservationID)
		if proof.State == "succeeded" {
			require.ErrorIs(t, deleteErr, storecenter.ErrVersionConflict)
			require.EqualValues(t, 2, proof.Snapshot.StoreVersion)
			require.NotNil(t, proof.Snapshot.ServiceState.ExpiresAt)
			deleteRequest.OperationKey = uuid.NewString()
			deleteRequest.ExpectedVersion = proof.Snapshot.StoreVersion
			_, err = admin.DeleteRecord(ctx, deleteRequest, time.Now().UTC())
			require.NoError(t, err)
		} else {
			require.Equal(t, "failed_fenced", proof.State)
			require.NoError(t, deleteErr)
		}
		persisted, err := repo.ReadServiceChargeProof(ctx, "org-a", intent.Execution.OperationID)
		require.NoError(t, err)
		require.Equal(t, proof, persisted)
		late, err := repo.ApplyServiceCharge(ctx, intent, storecenter.ConnectionStatusConnected)
		require.NoError(t, err)
		require.Equal(t, persisted, late)
		_, err = repo.Get(ctx, "org-a", created.ID())
		require.ErrorIs(t, err, storecenter.ErrNotFound)
	}
	// Old schema metadata is rejected, never migrated or silently accepted.
	require.NoError(t, owner.Exec("ALTER TABLE workbench_stores ADD COLUMN quota_allocation_id CHAR(36)").Error)
	require.Error(t, storecenter.VerifyCurrentSchema(ctx, owner))
}
