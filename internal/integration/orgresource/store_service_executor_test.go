package orgresourceadapter

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/storecenter"
)

func TestStoreServiceExecutorActivatesAndConsumesInOneTransaction(t *testing.T) {
	db, storeRepository := openStoreServiceTestDB(t)
	store := seedPendingActivationStore(t, db, storeRepository, "org-a", "00000000-0000-4000-8000-000000000501")
	seedResourceBucket(t, db, "org-a", 1)
	executor, err := NewStoreServiceExecutor(db, TransactionConfig{}, storeRepository)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	execution := storecenter.ServiceExecution{
		OrganizationID: "org-a", OperationID: "operation-activate", StoreID: store.ID(),
		Command: storecenter.ServiceCommandActivate, Quantity: 1, MaxQuantity: 12,
		ExpectedStoreVersion: 2, ExpectedConnectionRef: store.ConnectionRef(), ConnectionStatus: storecenter.ConnectionStatusConnected,
		ActorSubject: "operator", OccurredAt: now, RequestFingerprint: sixtyFourHex('a'),
	}

	first, err := executor.ExecuteServiceLifecycle(context.Background(), execution)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := executor.ExecuteServiceLifecycle(context.Background(), execution)
	if err != nil {
		t.Fatal(err)
	}
	if first.Replayed || !replayed.Replayed || replayed.Snapshot.OperationID != first.Snapshot.OperationID {
		t.Fatalf("results first=%+v replay=%+v", first, replayed)
	}
	if first.Snapshot.BalanceAfter != "0" || first.Snapshot.StoreVersion != 3 || first.Snapshot.ServiceState.ServiceStatus != storecenter.ServiceStatusActive {
		t.Fatalf("snapshot = %+v", first.Snapshot)
	}
	assertResourceBucket(t, db, "org-a", 0, 1)
	assertTableCount(t, db, "saas_organization_resource_operations", 1)
	assertTableCount(t, db, "saas_organization_resource_events", 1)
	assertTableCount(t, db, "saas_organization_resource_audit_logs", 1)
	assertStoreServiceRow(t, db, store.ID(), 3, storecenter.ServiceStatusActive, now, now.Add(30*24*time.Hour))
}

func TestStoreServiceExecutorRollsBackStoreAndResourceWhenAuditFails(t *testing.T) {
	db, storeRepository := openStoreServiceTestDB(t)
	store := seedPendingActivationStore(t, db, storeRepository, "org-a", "00000000-0000-4000-8000-000000000511")
	seedResourceBucket(t, db, "org-a", 1)
	if err := db.Exec(`CREATE TRIGGER fail_store_service_audit
		BEFORE INSERT ON saas_organization_resource_audit_logs
		BEGIN SELECT RAISE(ABORT, 'audit unavailable'); END`).Error; err != nil {
		t.Fatal(err)
	}
	executor, err := NewStoreServiceExecutor(db, TransactionConfig{}, storeRepository)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	_, err = executor.ExecuteServiceLifecycle(context.Background(), storecenter.ServiceExecution{
		OrganizationID: "org-a", OperationID: "operation-rollback", StoreID: store.ID(),
		Command: storecenter.ServiceCommandActivate, Quantity: 1, MaxQuantity: 12,
		ExpectedStoreVersion: 2, ExpectedConnectionRef: store.ConnectionRef(), ConnectionStatus: storecenter.ConnectionStatusConnected,
		ActorSubject: "operator", OccurredAt: now, RequestFingerprint: sixtyFourHex('b'),
	})
	if err == nil {
		t.Fatal("ExecuteServiceLifecycle() error = nil, want audit failure")
	}
	assertResourceBucket(t, db, "org-a", 1, 0)
	assertTableCount(t, db, "saas_organization_resource_operations", 0)
	assertTableCount(t, db, "saas_organization_resource_events", 0)
	assertTableCount(t, db, "saas_organization_resource_audit_logs", 0)
	assertStoreServiceRow(t, db, store.ID(), 2, storecenter.ServiceStatusPendingActivation, time.Time{}, time.Time{})
}

func TestStoreServiceExecutorReplaysTerminalInsufficientBalance(t *testing.T) {
	db, storeRepository := openStoreServiceTestDB(t)
	store := seedPendingActivationStore(t, db, storeRepository, "org-a", "00000000-0000-4000-8000-000000000521")
	seedResourceBucket(t, db, "org-a", 0)
	executor, err := NewStoreServiceExecutor(db, TransactionConfig{}, storeRepository)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	execution := storecenter.ServiceExecution{
		OrganizationID: "org-a", OperationID: "operation-insufficient", StoreID: store.ID(),
		Command: storecenter.ServiceCommandActivate, Quantity: 1, MaxQuantity: 12,
		ExpectedStoreVersion: 2, ExpectedConnectionRef: store.ConnectionRef(), ConnectionStatus: storecenter.ConnectionStatusConnected,
		ActorSubject: "operator", OccurredAt: now, RequestFingerprint: sixtyFourHex('c'),
	}
	if _, err := executor.ExecuteServiceLifecycle(context.Background(), execution); !errors.Is(err, orgresource.ErrInsufficientBalance) {
		t.Fatalf("first insufficient error = %v", err)
	}
	if err := db.Model(&organizationResourceBucketRow{}).
		Where("organization_id = ? AND resource_type = ?", "org-a", orgresource.ResourceStoreRenewalPeriod).
		Update("available", 1).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := executor.ExecuteServiceLifecycle(context.Background(), execution); !errors.Is(err, orgresource.ErrInsufficientBalance) {
		t.Fatalf("terminal replay error = %v", err)
	}
	assertResourceBucket(t, db, "org-a", 1, 0)
	assertStoreServiceRow(t, db, store.ID(), 2, storecenter.ServiceStatusPendingActivation, time.Time{}, time.Time{})
	assertTableCount(t, db, "saas_organization_resource_operations", 1)
	assertTableCount(t, db, "saas_organization_resource_events", 0)
	assertTableCount(t, db, "saas_organization_resource_audit_logs", 1)
}

func TestStoreServiceExecutorRecoversSuccessfulCommitResponseLoss(t *testing.T) {
	db, storeRepository := openStoreServiceTestDB(t)
	store := seedPendingActivationStore(t, db, storeRepository, "org-a", "00000000-0000-4000-8000-000000000531")
	seedResourceBucket(t, db, "org-a", 1)
	executor, err := NewStoreServiceExecutor(db, TransactionConfig{}, storeRepository)
	if err != nil {
		t.Fatal(err)
	}
	executor.afterCommit = func() error { return errSyntheticCommitResponseLoss }
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	result, err := executor.ExecuteServiceLifecycle(context.Background(), storecenter.ServiceExecution{
		OrganizationID: "org-a", OperationID: "operation-response-loss", StoreID: store.ID(),
		Command: storecenter.ServiceCommandActivate, Quantity: 1, MaxQuantity: 12,
		ExpectedStoreVersion: 2, ExpectedConnectionRef: store.ConnectionRef(), ConnectionStatus: storecenter.ConnectionStatusConnected,
		ActorSubject: "operator", OccurredAt: now, RequestFingerprint: sixtyFourHex('d'),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Replayed || result.Snapshot.StoreVersion != 3 {
		t.Fatalf("response-loss result = %+v", result)
	}
	assertResourceBucket(t, db, "org-a", 0, 1)
	assertTableCount(t, db, "saas_organization_resource_events", 1)
}

func TestStoreServiceExecutorClearsTerminalFailureBetweenTransactionRetries(t *testing.T) {
	db, storeRepository := openStoreServiceTestDB(t)
	store := seedPendingActivationStore(t, db, storeRepository, "org-a", "00000000-0000-4000-8000-000000000541")
	seedResourceBucket(t, db, "org-a", 1)
	stores := &retryingStoreServiceStore{delegate: storeRepository}
	executor, err := NewStoreServiceExecutor(db, TransactionConfig{}, stores)
	if err != nil {
		t.Fatal(err)
	}
	executor.runner = &rollbackOnceTransactionRunner{db: db}
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	result, err := executor.ExecuteServiceLifecycle(context.Background(), storecenter.ServiceExecution{
		OrganizationID: "org-a", OperationID: "operation-retry-terminal", StoreID: store.ID(),
		Command: storecenter.ServiceCommandActivate, Quantity: 1, MaxQuantity: 12,
		ExpectedStoreVersion: 2, ExpectedConnectionRef: store.ConnectionRef(), ConnectionStatus: storecenter.ConnectionStatusConnected,
		ActorSubject: "operator", OccurredAt: now, RequestFingerprint: sixtyFourHex('e'),
	})
	if err != nil {
		t.Fatalf("ExecuteServiceLifecycle() error = %v, want retry success", err)
	}
	if result.Replayed || result.Snapshot.StoreVersion != 3 {
		t.Fatalf("retry result = %+v, want committed second attempt", result)
	}
	if stores.lockCalls != 2 {
		t.Fatalf("LockServiceState calls = %d, want 2 attempts", stores.lockCalls)
	}
	assertResourceBucket(t, db, "org-a", 0, 1)
	assertStoreServiceRow(t, db, store.ID(), 3, storecenter.ServiceStatusActive, now, now.Add(30*24*time.Hour))
	assertTableCount(t, db, "saas_organization_resource_operations", 1)
}

type retryingStoreServiceStore struct {
	delegate  *storecenter.GormStoreRepository
	lockCalls int
}

func (s *retryingStoreServiceStore) LockServiceState(ctx context.Context, tx *gorm.DB, identity storecenter.ServiceStoreIdentity) (storecenter.ServiceStoreSnapshot, error) {
	s.lockCalls++
	snapshot, err := s.delegate.LockServiceState(ctx, tx, identity)
	if err == nil && s.lockCalls == 1 {
		snapshot.Version++
	}
	return snapshot, err
}

func (s *retryingStoreServiceStore) ApplyServiceState(ctx context.Context, tx *gorm.DB, mutation storecenter.ServiceStoreMutation) error {
	return s.delegate.ApplyServiceState(ctx, tx, mutation)
}

type rollbackOnceTransactionRunner struct {
	db       *gorm.DB
	attempts int
}

func (r *rollbackOnceTransactionRunner) run(ctx context.Context, operation func(*gorm.DB) error) error {
	r.attempts++
	if r.attempts == 1 {
		tx := r.db.WithContext(ctx).Begin()
		if tx.Error != nil {
			return tx.Error
		}
		if err := operation(tx); err != nil {
			_ = tx.Rollback().Error
			return err
		}
		if err := tx.Rollback().Error; err != nil {
			return err
		}
		return r.db.WithContext(ctx).Transaction(operation)
	}
	return r.db.WithContext(ctx).Transaction(operation)
}

func (r *rollbackOnceTransactionRunner) runRead(ctx context.Context, operation func(context.Context) error) error {
	return operation(ctx)
}

func openStoreServiceTestDB(t *testing.T) (*gorm.DB, *storecenter.GormStoreRepository) {
	t.Helper()
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if err := storecenter.AutoMigrateStoreRepository(db); err != nil {
		t.Fatal(err)
	}
	repository, err := storecenter.NewGormStoreRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	return db, repository
}

func seedPendingActivationStore(t *testing.T, db *gorm.DB, repository *storecenter.GormStoreRepository, organizationID, storeID string) *storecenter.Store {
	t.Helper()
	createdAt := time.Date(2026, 9, 4, 9, 0, 0, 0, time.UTC)
	store, err := storecenter.NewStore(storecenter.CreateStoreInput{
		ID: storeID, OrganizationID: organizationID, ActorSubject: "creator", Name: "Service Store", Platform: "shein", Region: "SG",
		ExternalStoreID: "external-" + storeID, CreateIdempotencyKey: "00000000-0000-4000-8000-000000000512",
		QuotaAllocationID: "00000000-0000-4000-8000-000000000513", OccurredAt: createdAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	store, _, err = repository.CreateOrReplay(context.Background(), organizationID, store)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.TransitionTo(storecenter.StoreStatusActive, "creator", createdAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repository.Save(context.Background(), organizationID, store, 1); err != nil {
		t.Fatal(err)
	}
	return store
}

func seedResourceBucket(t *testing.T, db *gorm.DB, organizationID string, available int64) {
	t.Helper()
	if err := db.Create(&organizationResourceBucketRow{OrganizationID: organizationID, ResourceType: string(orgresource.ResourceStoreRenewalPeriod), Available: available}).Error; err != nil {
		t.Fatal(err)
	}
}

func assertResourceBucket(t *testing.T, db *gorm.DB, organizationID string, available, consumed int64) {
	t.Helper()
	var row organizationResourceBucketRow
	if err := db.Where("organization_id = ? AND resource_type = ?", organizationID, orgresource.ResourceStoreRenewalPeriod).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Available != available || row.Consumed != consumed || row.Reserved != 0 {
		t.Fatalf("bucket = %+v, want available=%d consumed=%d reserved=0", row, available, consumed)
	}
}

func assertStoreServiceRow(t *testing.T, db *gorm.DB, storeID string, version int64, status storecenter.ServiceStatus, startedAt, expiresAt time.Time) {
	t.Helper()
	var row struct {
		Version          int64      `gorm:"column:version"`
		ServiceStatus    *string    `gorm:"column:service_status"`
		ServiceStartedAt *time.Time `gorm:"column:service_started_at"`
		ServiceExpiresAt *time.Time `gorm:"column:service_expires_at"`
	}
	if err := db.Table("workbench_stores").Where("id = ?", storeID).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Version != version || row.ServiceStatus == nil || *row.ServiceStatus != string(status) {
		t.Fatalf("store service row = %+v", row)
	}
	if startedAt.IsZero() {
		if row.ServiceStartedAt != nil || row.ServiceExpiresAt != nil {
			t.Fatalf("store service period = %+v, want nil", row)
		}
		return
	}
	if row.ServiceStartedAt == nil || !row.ServiceStartedAt.Equal(startedAt) || row.ServiceExpiresAt == nil || !row.ServiceExpiresAt.Equal(expiresAt) {
		t.Fatalf("store service period = %+v, want [%s,%s)", row, startedAt, expiresAt)
	}
}

func sixtyFourHex(value byte) string {
	bytes := make([]byte, 64)
	for i := range bytes {
		bytes[i] = value
	}
	return string(bytes)
}
