package orgresourceadapter

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	_ "modernc.org/sqlite"

	"task-processor/internal/ledger/orgresource"
)

func TestWelcomeGrantIsExactlyOnceAcrossOperationAndSourceReplay(t *testing.T) {
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repository, err := NewGormRepository(db, TransactionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	verifier := &mutableEligibilityVerifier{approval: orgresource.WelcomeGrantApproval{
		OrganizationID: "org-a",
		EvidenceID:     "bootstrap:org-a:v1",
		ApprovedAt:     time.Date(2026, 9, 4, 1, 2, 3, 0, time.UTC),
	}}
	service, err := orgresource.NewService(repository, verifier, allowWelcomeGrantAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	principal := orgresource.Principal{ID: "onboarding-worker", Kind: orgresource.PrincipalTrustedProvisioning}

	first, err := service.GrantWelcomeStoreRenewalPeriod(context.Background(), orgresource.GrantWelcomeStoreRenewalPeriodInput{
		OrganizationID: "org-a", OperationID: "operation-a", Principal: principal,
	})
	if err != nil {
		t.Fatal(err)
	}
	sameOperation, err := service.GrantWelcomeStoreRenewalPeriod(context.Background(), orgresource.GrantWelcomeStoreRenewalPeriodInput{
		OrganizationID: "org-a", OperationID: "operation-a", Principal: principal,
	})
	if err != nil {
		t.Fatal(err)
	}
	sameSource, err := service.GrantWelcomeStoreRenewalPeriod(context.Background(), orgresource.GrantWelcomeStoreRenewalPeriodInput{
		OrganizationID: "org-a", OperationID: "operation-b", Principal: principal,
	})
	if err != nil {
		t.Fatal(err)
	}

	if first.Replayed || !sameOperation.Replayed || !sameSource.Replayed {
		t.Fatalf("replay flags first=%v sameOperation=%v sameSource=%v", first.Replayed, sameOperation.Replayed, sameSource.Replayed)
	}
	if sameOperation.Snapshot != first.Snapshot || sameSource.Snapshot != first.Snapshot {
		t.Fatalf("snapshots differ: first=%#v operation=%#v source=%#v", first.Snapshot, sameOperation.Snapshot, sameSource.Snapshot)
	}
	if first.Snapshot.Quantity != "1" || first.Snapshot.BalanceAfter != "1" {
		t.Fatalf("snapshot = %#v", first.Snapshot)
	}

	assertTableCount(t, db, "saas_organization_resource_operations", 1)
	assertTableCount(t, db, "saas_organization_resource_source_claims", 1)
	assertTableCount(t, db, "saas_organization_resource_events", 1)
	assertTableCount(t, db, "saas_organization_resource_audit_logs", 1)
	var available int64
	if err := db.Table("saas_organization_resource_buckets").Select("available").Where("organization_id = ? AND resource_type = ?", "org-a", orgresource.ResourceStoreRenewalPeriod).Scan(&available).Error; err != nil {
		t.Fatal(err)
	}
	if available != 1 {
		t.Fatalf("available = %d, want 1", available)
	}
}

func TestWelcomeGrantReplaysSuccessfulSourceBeforeEligibilityChanges(t *testing.T) {
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repository, err := NewGormRepository(db, TransactionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	verifier := &mutableEligibilityVerifier{approval: orgresource.WelcomeGrantApproval{OrganizationID: "org-a", EvidenceID: "evidence-v1"}}
	service, err := orgresource.NewService(repository, verifier, allowWelcomeGrantAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	principal := orgresource.Principal{ID: "onboarding-worker", Kind: orgresource.PrincipalTrustedProvisioning}
	if _, err := service.GrantWelcomeStoreRenewalPeriod(context.Background(), orgresource.GrantWelcomeStoreRenewalPeriodInput{
		OrganizationID: "org-a", OperationID: "operation-a", Principal: principal,
	}); err != nil {
		t.Fatal(err)
	}

	verifier.approval.EvidenceID = "evidence-v2"
	for _, operationID := range []string{"operation-a", "operation-b"} {
		result, err := service.GrantWelcomeStoreRenewalPeriod(context.Background(), orgresource.GrantWelcomeStoreRenewalPeriodInput{
			OrganizationID: "org-a", OperationID: operationID, Principal: principal,
		})
		if err != nil {
			t.Fatalf("operation %s error = %v", operationID, err)
		}
		if !result.Replayed || result.Snapshot.BalanceAfter != "1" {
			t.Fatalf("operation %s result = %#v", operationID, result)
		}
	}
	assertTableCount(t, db, "saas_organization_resource_events", 1)
}

func TestWelcomeGrantRepositoryRejectsChangedFingerprintForOperationOrSource(t *testing.T) {
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repository, err := NewGormRepository(db, TransactionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := orgresource.NewService(repository, &mutableEligibilityVerifier{approval: orgresource.WelcomeGrantApproval{
		OrganizationID: "org-a", EvidenceID: "evidence-v1",
	}}, allowWelcomeGrantAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.GrantWelcomeStoreRenewalPeriod(context.Background(), orgresource.GrantWelcomeStoreRenewalPeriodInput{
		OrganizationID: "org-a",
		OperationID:    "operation-a",
		Principal:      orgresource.Principal{ID: "onboarding", Kind: orgresource.PrincipalTrustedProvisioning},
	}); err != nil {
		t.Fatal(err)
	}

	for _, operationID := range []string{"operation-a", "operation-b"} {
		_, err := repository.ExecuteWelcomeGrant(context.Background(), orgresource.WelcomeGrantExecution{
			OrganizationID:     "org-a",
			OperationID:        operationID,
			OperationType:      orgresource.OperationGrantWelcomeStoreRenewalPeriod,
			ResourceType:       orgresource.ResourceStoreRenewalPeriod,
			Quantity:           1,
			SourceType:         orgresource.SourceOnboardingWelcomeStorePeriod,
			SourceIdentity:     "org-a",
			ApprovalEvidenceID: "evidence-v2",
			ActorID:            "onboarding",
			RequestFingerprint: "changed-fingerprint",
		})
		if !errors.Is(err, orgresource.ErrIdempotencyKeyConflict) {
			t.Fatalf("operation %s error = %v, want ErrIdempotencyKeyConflict", operationID, err)
		}
	}
}

func TestWelcomeGrantRecoversFromCommitResponseLossByReadBack(t *testing.T) {
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repository, err := NewGormRepository(db, TransactionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	repository.afterCommit = func() error { return errSyntheticCommitResponseLoss }
	service, err := orgresource.NewService(repository, &mutableEligibilityVerifier{approval: orgresource.WelcomeGrantApproval{
		OrganizationID: "org-a", EvidenceID: "bootstrap:org-a:v1",
	}}, allowWelcomeGrantAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.GrantWelcomeStoreRenewalPeriod(context.Background(), orgresource.GrantWelcomeStoreRenewalPeriodInput{
		OrganizationID: "org-a",
		OperationID:    "operation-a",
		Principal:      orgresource.Principal{ID: "onboarding-worker", Kind: orgresource.PrincipalTrustedProvisioning},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Replayed || result.Snapshot.BalanceAfter != "1" {
		t.Fatalf("result = %#v", result)
	}
	assertTableCount(t, db, "saas_organization_resource_events", 1)
}

func TestWelcomeGrantRollsBackEveryPersistentEffectWhenAuditFails(t *testing.T) {
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TRIGGER fail_welcome_grant_audit
		BEFORE INSERT ON saas_organization_resource_audit_logs
		BEGIN SELECT RAISE(ABORT, 'audit unavailable'); END`).Error; err != nil {
		t.Fatal(err)
	}
	repository, err := NewGormRepository(db, TransactionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := orgresource.NewService(repository, &mutableEligibilityVerifier{approval: orgresource.WelcomeGrantApproval{
		OrganizationID: "org-a", EvidenceID: "bootstrap:org-a:v1",
	}}, allowWelcomeGrantAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.GrantWelcomeStoreRenewalPeriod(context.Background(), orgresource.GrantWelcomeStoreRenewalPeriodInput{
		OrganizationID: "org-a",
		OperationID:    "operation-a",
		Principal:      orgresource.Principal{ID: "onboarding", Kind: orgresource.PrincipalTrustedProvisioning},
	})
	if err == nil {
		t.Fatal("GrantWelcomeStoreRenewalPeriod() error = nil, want audit failure")
	}

	for _, table := range []string{
		"saas_organization_resource_buckets",
		"saas_organization_resource_debts",
		"saas_organization_resource_operations",
		"saas_organization_resource_source_claims",
		"saas_organization_resource_events",
		"saas_organization_resource_audit_logs",
	} {
		assertTableCount(t, db, table, 0)
	}
}

func TestWelcomeGrantRepaysResourceDebtBeforeIncreasingAvailable(t *testing.T) {
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&organizationResourceBucketRow{
		OrganizationID: "org-a", ResourceType: string(orgresource.ResourceStoreRenewalPeriod),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&organizationResourceDebtRow{
		OrganizationID: "org-a", ResourceType: string(orgresource.ResourceStoreRenewalPeriod), Amount: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	repository, err := NewGormRepository(db, TransactionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := orgresource.NewService(repository, &mutableEligibilityVerifier{approval: orgresource.WelcomeGrantApproval{
		OrganizationID: "org-a", EvidenceID: "bootstrap:org-a:v1",
	}}, allowWelcomeGrantAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.GrantWelcomeStoreRenewalPeriod(context.Background(), orgresource.GrantWelcomeStoreRenewalPeriodInput{
		OrganizationID: "org-a", OperationID: "operation-a",
		Principal: orgresource.Principal{ID: "onboarding", Kind: orgresource.PrincipalTrustedProvisioning},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Snapshot.BalanceAfter != "0" {
		t.Fatalf("result = %#v", result)
	}
	assertBucket(t, db, "org-a", orgresource.ResourceStoreRenewalPeriod, 0, 0, 0)
	assertDebt(t, db, "org-a", orgresource.ResourceStoreRenewalPeriod, 0)
	var event organizationResourceEventRow
	if err := db.Where("organization_id = ? AND operation_id = ?", "org-a", "operation-a").Take(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.GrossCredit != 1 || event.DebtRepaid != 1 || event.NetCredit != 0 || event.AvailableDelta != 0 {
		t.Fatalf("event = %#v", event)
	}
}

func TestResourceBucketRejectsNegativeBalances(t *testing.T) {
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	err := db.Exec(`INSERT INTO saas_organization_resource_buckets
		(organization_id, resource_type, available, reserved, consumed, created_at, updated_at)
		VALUES (?, ?, -1, 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, "org-a", orgresource.ResourceStoreRenewalPeriod).Error
	if err == nil {
		t.Fatal("negative available balance was accepted")
	}
}

func TestResourceEventRequiresOrganizationScopedOperation(t *testing.T) {
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatal(err)
	}
	err := db.Exec(`INSERT INTO saas_organization_resource_events
		(event_id, organization_id, operation_id, resource_type, quantity,
		 available_delta, reserved_delta, consumed_delta, reason, source_type,
		 source_identity, balance_after, created_at)
		VALUES (?, ?, ?, ?, 1, 1, 0, 0, ?, ?, ?, 1, CURRENT_TIMESTAMP)`,
		"event-orphan", "org-b", "operation-from-org-a", orgresource.ResourceStoreRenewalPeriod,
		orgresource.SourceOnboardingWelcomeStorePeriod, orgresource.SourceOnboardingWelcomeStorePeriod, "org-b").Error
	if err == nil {
		t.Fatal("event without a same-organization operation was accepted")
	}
}

func TestWelcomeGrantConcurrentCallsMintOnce(t *testing.T) {
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repository, err := NewGormRepository(db, TransactionConfig{MaxAttempts: 8, TotalRetryBudget: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	service, err := orgresource.NewService(repository, &mutableEligibilityVerifier{approval: orgresource.WelcomeGrantApproval{
		OrganizationID: "org-a", EvidenceID: "bootstrap:org-a:v1",
	}}, allowWelcomeGrantAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	principal := orgresource.Principal{ID: "onboarding-worker", Kind: orgresource.PrincipalTrustedProvisioning}

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, callErr := service.GrantWelcomeStoreRenewalPeriod(context.Background(), orgresource.GrantWelcomeStoreRenewalPeriodInput{
				OrganizationID: "org-a", OperationID: "operation-concurrent-" + string(rune('a'+index)), Principal: principal,
			})
			errs <- callErr
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent grant: %v", err)
		}
	}
	assertTableCount(t, db, "saas_organization_resource_events", 1)
}

var errSyntheticCommitResponseLoss = errors.New("synthetic commit response loss")

type mutableEligibilityVerifier struct {
	mu       sync.Mutex
	approval orgresource.WelcomeGrantApproval
}

type allowWelcomeGrantAuthorizer struct{}

func (allowWelcomeGrantAuthorizer) AuthorizeWelcomeGrant(context.Context, orgresource.Principal) error {
	return nil
}

func (v *mutableEligibilityVerifier) VerifyWelcomeGrantEligibility(context.Context, string) (orgresource.WelcomeGrantApproval, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.approval, nil
}

func openSQLiteStore(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "orgresource.db")) + "?mode=rwc&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: dsn}, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func assertTableCount(t *testing.T, db *gorm.DB, table string, want int64) {
	t.Helper()
	var got int64
	if err := db.Table(table).Count(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s count = %d, want %d", table, got, want)
	}
}
