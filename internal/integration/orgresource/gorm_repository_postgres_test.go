//go:build integration

package orgresourceadapter

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/storecenter"
)

func TestPostgresWelcomeGrantConcurrentSourceReplayMintsOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("orgresource"),
		tcpostgres.WithUsername("orgresource"),
		tcpostgres.WithPassword("orgresource"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start PostgreSQL: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repository, err := NewGormRepository(db, TransactionConfig{
		LockTimeout:        time.Second,
		StatementTimeout:   3 * time.Second,
		TransactionTimeout: 5 * time.Second,
		TotalRetryBudget:   20 * time.Second,
		MaxAttempts:        12,
	})
	if err != nil {
		t.Fatal(err)
	}
	service, err := orgresource.NewService(repository, &mutableEligibilityVerifier{approval: orgresource.WelcomeGrantApproval{
		OrganizationID: "org-postgres",
		EvidenceID:     "bootstrap:org-postgres:v1",
		ApprovedAt:     time.Date(2026, 9, 4, 1, 2, 3, 0, time.UTC),
	}}, allowWelcomeGrantAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	principal := orgresource.Principal{ID: "onboarding-worker", Kind: orgresource.PrincipalTrustedProvisioning}

	const callers = 12
	results := make(chan orgresource.WelcomeGrantResult, callers)
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for index := 0; index < callers; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			result, callErr := service.GrantWelcomeStoreRenewalPeriod(ctx, orgresource.GrantWelcomeStoreRenewalPeriodInput{
				OrganizationID: "org-postgres",
				OperationID:    fmt.Sprintf("operation-%02d", index),
				Principal:      principal,
			})
			results <- result
			errs <- callErr
		}(index)
	}
	wg.Wait()
	close(results)
	close(errs)
	for callErr := range errs {
		if callErr != nil {
			t.Fatalf("concurrent welcome grant: %v", callErr)
		}
	}
	fresh := 0
	for result := range results {
		if !result.Replayed {
			fresh++
		}
		if result.Snapshot.BalanceAfter != "1" || result.Snapshot.Quantity != "1" {
			t.Fatalf("result = %#v", result)
		}
	}
	if fresh != 1 {
		t.Fatalf("fresh grants = %d, want 1", fresh)
	}
	assertTableCount(t, db, "saas_organization_resource_operations", 1)
	assertTableCount(t, db, "saas_organization_resource_source_claims", 1)
	assertTableCount(t, db, "saas_organization_resource_events", 1)
	assertTableCount(t, db, "saas_organization_resource_audit_logs", 1)
	var available int64
	if err := db.Table("saas_organization_resource_buckets").Select("available").
		Where("organization_id = ? AND resource_type = ?", "org-postgres", orgresource.ResourceStoreRenewalPeriod).
		Scan(&available).Error; err != nil {
		t.Fatal(err)
	}
	if available != 1 {
		t.Fatalf("available = %d, want 1", available)
	}

	if err := db.AutoMigrate(&testOwnerAttemptRow{}); err != nil {
		t.Fatal(err)
	}
	seedResourceBalance(t, db, "org-postgres-reserve", orgresource.ResourceAIPoint, 1)
	seedOwnerAttempt(t, db, testOwnerAttemptRow{
		OrganizationID: "org-postgres-reserve", OwnerType: "listingkit_generation", AttemptID: "attempt-a",
		BusinessScope: "listing-kit:task-a", State: string(orgresource.OwnerAttemptNotStarted),
	})
	reservationRepository, err := NewGormReservationRepository(db, TransactionConfig{
		LockTimeout: time.Second, StatementTimeout: 3 * time.Second, TransactionTimeout: 5 * time.Second,
		TotalRetryBudget: 20 * time.Second, MaxAttempts: 12,
	}, map[string]TransactionalReservationOwnerStore{"listingkit_generation": &testOwnerStore{}})
	if err != nil {
		t.Fatal(err)
	}
	reservationService, err := orgresource.NewReservationService(reservationRepository, allowReservationAuthorizer{maxQuantity: 1})
	if err != nil {
		t.Fatal(err)
	}
	reservationResults := make(chan orgresource.ReservationResult, callers)
	reservationErrors := make(chan error, callers)
	for index := 0; index < callers; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			input := validRepositoryReserveInput()
			input.OrganizationID = "org-postgres-reserve"
			input.OperationID = fmt.Sprintf("reserve-operation-%02d", index)
			result, reserveErr := reservationService.Reserve(ctx, input)
			reservationResults <- result
			reservationErrors <- reserveErr
		}(index)
	}
	wg.Wait()
	close(reservationResults)
	close(reservationErrors)
	for reserveErr := range reservationErrors {
		if reserveErr != nil {
			t.Fatalf("concurrent reservation: %v", reserveErr)
		}
	}
	freshReservations := 0
	firstReservationID := ""
	for result := range reservationResults {
		if !result.Replayed {
			freshReservations++
		}
		if result.Snapshot.AvailableAfter != "0" || result.Snapshot.ReservedAfter != "1" {
			t.Fatalf("reservation result = %#v", result)
		}
		if firstReservationID == "" {
			firstReservationID = result.Snapshot.ReservationID
		}
	}
	if freshReservations != 1 {
		t.Fatalf("fresh reservations = %d, want 1", freshReservations)
	}
	assertTableCount(t, db, "saas_organization_resource_reservations", 1)
	assertBucket(t, db, "org-postgres-reserve", orgresource.ResourceAIPoint, 0, 1, 0)
	if err := db.Model(&testOwnerAttemptRow{}).
		Where("organization_id = ? AND owner_type = ? AND attempt_id = ?", "org-postgres-reserve", "listingkit_generation", "attempt-a").
		Updates(map[string]any{"state": orgresource.OwnerAttemptSucceededTerminal, "terminal_evidence_id": "provider-receipt-a"}).Error; err != nil {
		t.Fatal(err)
	}
	settlementService, err := orgresource.NewSettlementService(reservationRepository, allowSettlementAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	settlementResults := make(chan orgresource.SettlementResult, callers)
	settlementErrors := make(chan error, callers)
	for index := 0; index < callers; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			input := settlementInput("", fmt.Sprintf("settle-operation-%02d", index))
			input.OrganizationID = "org-postgres-reserve"
			input.ReservationID = firstReservationID
			result, settleErr := settlementService.Settle(ctx, input)
			settlementResults <- result
			settlementErrors <- settleErr
		}(index)
	}
	wg.Wait()
	close(settlementResults)
	close(settlementErrors)
	for settleErr := range settlementErrors {
		if settleErr != nil {
			t.Fatalf("concurrent settlement: %v", settleErr)
		}
	}
	freshSettlements := 0
	for result := range settlementResults {
		if !result.Replayed {
			freshSettlements++
		}
		if result.Snapshot.Decision != orgresource.SettlementCommit || result.Snapshot.ConsumedAfter != "1" {
			t.Fatalf("settlement result = %#v", result)
		}
	}
	if freshSettlements != 1 {
		t.Fatalf("fresh settlements = %d, want 1", freshSettlements)
	}
	assertBucket(t, db, "org-postgres-reserve", orgresource.ResourceAIPoint, 0, 0, 1)

	if err := storecenter.AutoMigrateStoreRepository(db); err != nil {
		t.Fatal(err)
	}
	storeRepository, err := storecenter.NewGormStoreRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	serviceStore := seedPendingActivationStore(t, db, storeRepository, "org-postgres-store", "00000000-0000-4000-8000-000000000541")
	seedResourceBucket(t, db, "org-postgres-store", 1)
	storeServiceExecutor, err := NewStoreServiceExecutor(db, TransactionConfig{
		LockTimeout: time.Second, StatementTimeout: 3 * time.Second, TransactionTimeout: 5 * time.Second,
		TotalRetryBudget: 20 * time.Second, MaxAttempts: 12,
	}, storeRepository)
	if err != nil {
		t.Fatal(err)
	}
	serviceExecution := storecenter.ServiceExecution{
		OrganizationID: "org-postgres-store", OperationID: "operation-concurrent-activate", StoreID: serviceStore.ID(),
		Command: storecenter.ServiceCommandActivate, Quantity: 1, MaxQuantity: 12,
		ExpectedStoreVersion: 2, ExpectedConnectionRef: serviceStore.ConnectionRef(), ConnectionStatus: storecenter.ConnectionStatusConnected,
		ActorSubject: "operator", OccurredAt: time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC), RequestFingerprint: sixtyFourHex('e'),
	}
	serviceResults := make(chan storecenter.ServiceOperationResult, callers)
	serviceErrors := make(chan error, callers)
	for index := 0; index < callers; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, executeErr := storeServiceExecutor.ExecuteServiceLifecycle(ctx, serviceExecution)
			serviceResults <- result
			serviceErrors <- executeErr
		}()
	}
	wg.Wait()
	close(serviceResults)
	close(serviceErrors)
	for executeErr := range serviceErrors {
		if executeErr != nil {
			t.Fatalf("concurrent Store activation: %v", executeErr)
		}
	}
	freshServiceOperations := 0
	for result := range serviceResults {
		if !result.Replayed {
			freshServiceOperations++
		}
		if result.Snapshot.StoreVersion != 3 || result.Snapshot.BalanceAfter != "0" {
			t.Fatalf("Store activation result = %+v", result)
		}
	}
	if freshServiceOperations != 1 {
		t.Fatalf("fresh Store service operations = %d, want 1", freshServiceOperations)
	}
	assertBucket(t, db, "org-postgres-store", orgresource.ResourceStoreRenewalPeriod, 0, 0, 1)
	var serviceEventCount int64
	if err := db.Table("saas_organization_resource_events").Where("organization_id = ?", "org-postgres-store").Count(&serviceEventCount).Error; err != nil {
		t.Fatal(err)
	}
	if serviceEventCount != 1 {
		t.Fatalf("Store service event count = %d, want 1", serviceEventCount)
	}
	assertStoreServiceRow(t, db, serviceStore.ID(), 3, storecenter.ServiceStatusActive, serviceExecution.OccurredAt, serviceExecution.OccurredAt.Add(30*24*time.Hour))
}

func init() {
	if runtime.GOOS == "windows" && os.Getenv("DOCKER_HOST") == "" {
		_ = os.Setenv("DOCKER_HOST", "npipe:////./pipe/dockerDesktopLinuxEngine")
	}
}

func TestPostgresResourceRuntimeMonthlyLimitPermissions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("orgresource"), tcpostgres.WithUsername("orgresource"), tcpostgres.WithPassword("orgresource"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	owner, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := owner.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = pool.Close() })
	require.NoError(t, AutoMigrate(owner))
	// This role exists only in this test's dedicated, disposable cluster.
	require.NoError(t, owner.Exec(`CREATE ROLE commercial_owner_runtime LOGIN PASSWORD 'controlled-test-only'`).Error)
	for _, statement := range []string{
		`REVOKE CREATE ON SCHEMA public FROM PUBLIC`,
		`GRANT USAGE ON SCHEMA public TO commercial_owner_runtime`,
		`GRANT SELECT,INSERT,UPDATE ON saas_organization_resource_buckets,saas_organization_resource_operations,saas_organization_resource_reservations,saas_organization_resource_debts,saas_member_ai_point_limits,saas_member_ai_point_months TO commercial_owner_runtime`,
		`GRANT SELECT,INSERT ON saas_organization_resource_source_claims,saas_organization_resource_events,saas_organization_resource_audit_logs TO commercial_owner_runtime`,
		`GRANT USAGE,SELECT ON SEQUENCE saas_organization_resource_audit_logs_id_seq TO commercial_owner_runtime`,
	} {
		require.NoError(t, owner.Exec(statement).Error)
	}
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.User = url.UserPassword(RuntimeRole, "controlled-test-only")
	runtimeDB, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	runtimePool, err := runtimeDB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = runtimePool.Close() })
	require.NoError(t, VerifyRuntimePermissions(ctx, runtimeDB))
	require.Error(t, VerifyRuntimePermissions(ctx, owner), "schema owner cannot run the application")
	require.NoError(t, owner.Exec(`REVOKE UPDATE ON saas_member_ai_point_months FROM commercial_owner_runtime`).Error)
	require.Error(t, VerifyRuntimePermissions(ctx, runtimeDB))
	require.NoError(t, owner.Exec(`GRANT UPDATE ON saas_member_ai_point_months TO commercial_owner_runtime`).Error)
	require.NoError(t, owner.Exec(`GRANT DELETE ON saas_member_ai_point_limits TO commercial_owner_runtime`).Error)
	require.Error(t, VerifyRuntimePermissions(ctx, runtimeDB))
	require.NoError(t, owner.Exec(`REVOKE DELETE ON saas_member_ai_point_limits FROM commercial_owner_runtime`).Error)
	require.NoError(t, VerifyRuntimePermissions(ctx, runtimeDB))
	require.NoError(t, runtimeDB.Exec(`INSERT INTO saas_member_ai_point_limits (organization_id,member_id,monthly_limit,version,updated_by,updated_at) VALUES ('org-1','member-1',25,1,'admin-1',now())`).Error)
	require.NoError(t, runtimeDB.Exec(`UPDATE saas_member_ai_point_limits SET monthly_limit=30 WHERE organization_id='org-1' AND member_id='member-1'`).Error)
	require.Error(t, runtimeDB.Exec(`DELETE FROM saas_member_ai_point_limits WHERE organization_id='org-1' AND member_id='member-1'`).Error)
	require.Error(t, runtimeDB.Exec(`CREATE TABLE forbidden_ddl(id integer)`).Error)
}
