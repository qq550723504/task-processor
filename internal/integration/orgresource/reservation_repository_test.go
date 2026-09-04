package orgresourceadapter

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/ledger/orgresource"
)

func TestReservationAtomicallyBindsOwnerAndMovesAvailableToReserved(t *testing.T) {
	db, repository, service := newReservationTestService(t)
	seedResourceBalance(t, db, "org-a", orgresource.ResourceAIPoint, 2)
	seedOwnerAttempt(t, db, testOwnerAttemptRow{
		OrganizationID: "org-a", OwnerType: "listingkit_generation", AttemptID: "attempt-a",
		BusinessScope: "listing-kit:task-a", State: string(orgresource.OwnerAttemptNotStarted),
	})

	result, err := service.Reserve(context.Background(), validRepositoryReserveInput())
	if err != nil {
		t.Fatal(err)
	}
	if result.Replayed || result.Snapshot.AvailableAfter != "1" || result.Snapshot.ReservedAfter != "1" || result.Snapshot.ConsumedAfter != "0" {
		t.Fatalf("result = %#v", result)
	}

	var owner testOwnerAttemptRow
	if err := db.Where("organization_id = ? AND owner_type = ? AND attempt_id = ?", "org-a", "listingkit_generation", "attempt-a").Take(&owner).Error; err != nil {
		t.Fatal(err)
	}
	if owner.ReservationID != result.Snapshot.ReservationID || owner.ResourceType != string(orgresource.ResourceAIPoint) || owner.ReservationPurpose != "generation" {
		t.Fatalf("owner binding = %#v", owner)
	}
	assertTableCount(t, db, "saas_organization_resource_reservations", 1)
	assertTableCount(t, db, "saas_organization_resource_operations", 2) // seed credit + reserve
	assertTableCount(t, db, "saas_organization_resource_events", 2)
	assertTableCount(t, db, "saas_organization_resource_audit_logs", 2)
	_ = repository
}

func TestReservationReplaysByOperationOrExactOwnerIdentity(t *testing.T) {
	db, _, service := newReservationTestService(t)
	seedResourceBalance(t, db, "org-a", orgresource.ResourceAIPoint, 2)
	seedOwnerAttempt(t, db, testOwnerAttemptRow{
		OrganizationID: "org-a", OwnerType: "listingkit_generation", AttemptID: "attempt-a",
		BusinessScope: "listing-kit:task-a", State: string(orgresource.OwnerAttemptNotStarted),
	})
	input := validRepositoryReserveInput()
	first, err := service.Reserve(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	sameOperation, err := service.Reserve(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	input.OperationID = "operation-b"
	sameOwner, err := service.Reserve(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !sameOperation.Replayed || !sameOwner.Replayed || sameOperation.Snapshot != first.Snapshot || sameOwner.Snapshot != first.Snapshot {
		t.Fatalf("first=%#v same operation=%#v same owner=%#v", first, sameOperation, sameOwner)
	}
	assertTableCount(t, db, "saas_organization_resource_reservations", 1)
	assertBucket(t, db, "org-a", orgresource.ResourceAIPoint, 1, 1, 0)
}

func TestReservationRejectsChangedOwnerPayloadAndTenantScope(t *testing.T) {
	db, _, service := newReservationTestService(t)
	seedResourceBalance(t, db, "org-a", orgresource.ResourceAIPoint, 2)
	seedOwnerAttempt(t, db, testOwnerAttemptRow{
		OrganizationID: "org-a", OwnerType: "listingkit_generation", AttemptID: "attempt-a",
		BusinessScope: "listing-kit:task-a", State: string(orgresource.OwnerAttemptNotStarted),
	})
	input := validRepositoryReserveInput()
	if _, err := service.Reserve(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*orgresource.ReserveInput){
		func(in *orgresource.ReserveInput) { in.Quantity = 2; in.OperationID = "operation-b" },
		func(in *orgresource.ReserveInput) {
			in.BusinessScope = "listing-kit:task-other"
			in.OperationID = "operation-c"
		},
		func(in *orgresource.ReserveInput) { in.OrganizationID = "org-b"; in.OperationID = "operation-d" },
		func(in *orgresource.ReserveInput) {
			in.ResourceType = orgresource.ResourceDataRow
			in.OperationID = "operation-e"
		},
		func(in *orgresource.ReserveInput) {
			in.ReservationPurpose = "different-purpose"
			in.OperationID = "operation-f"
		},
	} {
		changed := input
		mutate(&changed)
		_, err := service.Reserve(context.Background(), changed)
		if !errors.Is(err, orgresource.ErrIdempotencyKeyConflict) && !errors.Is(err, orgresource.ErrOwnerScopeMismatch) {
			t.Fatalf("changed input %#v error = %v", changed, err)
		}
	}
	assertBucket(t, db, "org-a", orgresource.ResourceAIPoint, 1, 1, 0)
}

func TestReservationRejectsLateOwnerAndInsufficientBalanceWithoutPartialWrites(t *testing.T) {
	for _, tt := range []struct {
		name      string
		state     orgresource.OwnerAttemptState
		available int64
		seedOps   int64
		want      error
	}{
		{name: "owner already processing", state: orgresource.OwnerAttemptProcessing, available: 1, seedOps: 1, want: orgresource.ErrOwnerNotReservable},
		{name: "insufficient resource", state: orgresource.OwnerAttemptNotStarted, available: 0, seedOps: 0, want: orgresource.ErrInsufficientBalance},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db, _, service := newReservationTestService(t)
			seedResourceBalance(t, db, "org-a", orgresource.ResourceAIPoint, tt.available)
			seedOwnerAttempt(t, db, testOwnerAttemptRow{
				OrganizationID: "org-a", OwnerType: "listingkit_generation", AttemptID: "attempt-a",
				BusinessScope: "listing-kit:task-a", State: string(tt.state),
			})
			_, err := service.Reserve(context.Background(), validRepositoryReserveInput())
			if !errors.Is(err, tt.want) {
				t.Fatalf("Reserve() error = %v, want %v", err, tt.want)
			}
			assertTableCount(t, db, "saas_organization_resource_reservations", 0)
			assertTableCount(t, db, "saas_organization_resource_operations", tt.seedOps)
			assertBucket(t, db, "org-a", orgresource.ResourceAIPoint, tt.available, 0, 0)
		})
	}
}

func TestReservationOwnerBindingFailureRollsBackLedgerEffects(t *testing.T) {
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&testOwnerAttemptRow{}); err != nil {
		t.Fatal(err)
	}
	ownerStore := &testOwnerStore{bindErr: errors.New("owner storage unavailable")}
	repository, err := NewGormReservationRepository(db, TransactionConfig{}, map[string]TransactionalReservationOwnerStore{"listingkit_generation": ownerStore})
	if err != nil {
		t.Fatal(err)
	}
	service, err := orgresource.NewReservationService(repository, allowReservationAuthorizer{maxQuantity: 10})
	if err != nil {
		t.Fatal(err)
	}
	seedResourceBalance(t, db, "org-a", orgresource.ResourceAIPoint, 1)
	seedOwnerAttempt(t, db, testOwnerAttemptRow{
		OrganizationID: "org-a", OwnerType: "listingkit_generation", AttemptID: "attempt-a",
		BusinessScope: "listing-kit:task-a", State: string(orgresource.OwnerAttemptNotStarted),
	})

	if _, err := service.Reserve(context.Background(), validRepositoryReserveInput()); err == nil {
		t.Fatal("Reserve() error = nil, want owner binding failure")
	}
	assertTableCount(t, db, "saas_organization_resource_reservations", 0)
	assertTableCount(t, db, "saas_organization_resource_operations", 1)
	assertBucket(t, db, "org-a", orgresource.ResourceAIPoint, 1, 0, 0)
}

func TestReservationRecoversFromCommitResponseLoss(t *testing.T) {
	db, repository, service := newReservationTestService(t)
	repository.afterCommit = func() error { return errSyntheticCommitResponseLoss }
	seedResourceBalance(t, db, "org-a", orgresource.ResourceAIPoint, 1)
	seedOwnerAttempt(t, db, testOwnerAttemptRow{
		OrganizationID: "org-a", OwnerType: "listingkit_generation", AttemptID: "attempt-a",
		BusinessScope: "listing-kit:task-a", State: string(orgresource.OwnerAttemptNotStarted),
	})

	result, err := service.Reserve(context.Background(), validRepositoryReserveInput())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Replayed || result.Snapshot.ReservationID == "" {
		t.Fatalf("result = %#v", result)
	}
	assertTableCount(t, db, "saas_organization_resource_reservations", 1)
	assertBucket(t, db, "org-a", orgresource.ResourceAIPoint, 0, 1, 0)
}

func TestReservationAuditFailureRollsBackOwnerBindingAndResourceMutation(t *testing.T) {
	db, _, service := newReservationTestService(t)
	seedResourceBalance(t, db, "org-a", orgresource.ResourceAIPoint, 1)
	seedOwnerAttempt(t, db, testOwnerAttemptRow{
		OrganizationID: "org-a", OwnerType: "listingkit_generation", AttemptID: "attempt-a",
		BusinessScope: "listing-kit:task-a", State: string(orgresource.OwnerAttemptNotStarted),
	})
	if err := db.Exec(`CREATE TRIGGER fail_reservation_audit
		BEFORE INSERT ON saas_organization_resource_audit_logs
		WHEN NEW.action = 'reserve'
		BEGIN SELECT RAISE(ABORT, 'audit unavailable'); END`).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := service.Reserve(context.Background(), validRepositoryReserveInput()); err == nil {
		t.Fatal("Reserve() error = nil, want audit failure")
	}
	var owner testOwnerAttemptRow
	if err := db.Where("organization_id = ? AND owner_type = ? AND attempt_id = ?", "org-a", "listingkit_generation", "attempt-a").Take(&owner).Error; err != nil {
		t.Fatal(err)
	}
	if owner.ReservationID != "" || owner.ResourceType != "" || owner.ReservationPurpose != "" {
		t.Fatalf("owner binding survived rollback: %#v", owner)
	}
	assertTableCount(t, db, "saas_organization_resource_reservations", 0)
	assertTableCount(t, db, "saas_organization_resource_operations", 1)
	assertBucket(t, db, "org-a", orgresource.ResourceAIPoint, 1, 0, 0)
}

func TestReservationRequiresRegisteredDurableOwnerAdapter(t *testing.T) {
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repository, err := NewGormReservationRepository(db, TransactionConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	service, err := orgresource.NewReservationService(repository, allowReservationAuthorizer{maxQuantity: 1})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Reserve(context.Background(), validRepositoryReserveInput())
	if !errors.Is(err, orgresource.ErrReservationOwnerNotRegistered) {
		t.Fatalf("Reserve() error = %v, want ErrReservationOwnerNotRegistered", err)
	}
}

type testOwnerAttemptRow struct {
	OrganizationID     string `gorm:"column:organization_id;primaryKey"`
	OwnerType          string `gorm:"column:owner_type;primaryKey"`
	AttemptID          string `gorm:"column:attempt_id;primaryKey"`
	BusinessScope      string `gorm:"column:business_scope;not null"`
	State              string `gorm:"column:state;not null"`
	ReservationID      string `gorm:"column:reservation_id"`
	ResourceType       string `gorm:"column:resource_type"`
	ReservationPurpose string `gorm:"column:reservation_purpose"`
	TerminalEvidenceID string `gorm:"column:terminal_evidence_id"`
}

func (testOwnerAttemptRow) TableName() string { return "test_resource_owner_attempts" }

type testOwnerStore struct {
	bindErr     error
	terminalErr error
}

func (store *testOwnerStore) LockOwnerAttempt(ctx context.Context, tx *gorm.DB, identity OwnerAttemptIdentity) (OwnerAttemptSnapshot, error) {
	var row testOwnerAttemptRow
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("organization_id = ? AND owner_type = ? AND attempt_id = ?", identity.OrganizationID, identity.OwnerType, identity.OwnerAttemptID).
		Take(&row).Error
	if err != nil {
		return OwnerAttemptSnapshot{}, err
	}
	return OwnerAttemptSnapshot{
		OrganizationID: row.OrganizationID, OwnerType: row.OwnerType, OwnerAttemptID: row.AttemptID,
		BusinessScope: row.BusinessScope, State: orgresource.OwnerAttemptState(row.State),
		ReservationID: row.ReservationID, ResourceType: orgresource.ResourceType(row.ResourceType), ReservationPurpose: row.ReservationPurpose,
	}, nil
}

func (store *testOwnerStore) BindReservation(ctx context.Context, tx *gorm.DB, binding OwnerReservationBinding) error {
	if store.bindErr != nil {
		return store.bindErr
	}
	return tx.WithContext(ctx).Model(&testOwnerAttemptRow{}).
		Where("organization_id = ? AND owner_type = ? AND attempt_id = ? AND state = ?", binding.OrganizationID, binding.OwnerType, binding.OwnerAttemptID, orgresource.OwnerAttemptNotStarted).
		Updates(map[string]any{"reservation_id": binding.ReservationID, "resource_type": binding.ResourceType, "reservation_purpose": binding.ReservationPurpose}).Error
}

func (store *testOwnerStore) LockTerminalProof(ctx context.Context, tx *gorm.DB, binding OwnerReservationBinding) (OwnerTerminalProof, error) {
	if store.terminalErr != nil {
		return OwnerTerminalProof{}, store.terminalErr
	}
	owner, err := store.LockOwnerAttempt(ctx, tx, OwnerAttemptIdentity{
		OrganizationID: binding.OrganizationID, OwnerType: binding.OwnerType, OwnerAttemptID: binding.OwnerAttemptID,
	})
	if err != nil {
		return OwnerTerminalProof{}, err
	}
	var row testOwnerAttemptRow
	if err := tx.WithContext(ctx).Where("organization_id = ? AND owner_type = ? AND attempt_id = ?", binding.OrganizationID, binding.OwnerType, binding.OwnerAttemptID).Take(&row).Error; err != nil {
		return OwnerTerminalProof{}, err
	}
	return OwnerTerminalProof{
		OrganizationID: owner.OrganizationID, OwnerType: owner.OwnerType, OwnerAttemptID: owner.OwnerAttemptID,
		BusinessScope: owner.BusinessScope, ReservationID: owner.ReservationID, ResourceType: owner.ResourceType,
		ReservationPurpose: owner.ReservationPurpose, State: owner.State, EvidenceID: row.TerminalEvidenceID,
	}, nil
}

type allowReservationAuthorizer struct{ maxQuantity int64 }

func (a allowReservationAuthorizer) AuthorizeReservation(context.Context, orgresource.Principal, string, orgresource.ResourceType) (orgresource.ReservationAuthorization, error) {
	return orgresource.ReservationAuthorization{MaxQuantity: a.maxQuantity}, nil
}

func newReservationTestService(t *testing.T) (*gorm.DB, *GormReservationRepository, *orgresource.ReservationService) {
	t.Helper()
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&testOwnerAttemptRow{}); err != nil {
		t.Fatal(err)
	}
	repository, err := NewGormReservationRepository(db, TransactionConfig{}, map[string]TransactionalReservationOwnerStore{
		"listingkit_generation": &testOwnerStore{},
	})
	if err != nil {
		t.Fatal(err)
	}
	service, err := orgresource.NewReservationService(repository, allowReservationAuthorizer{maxQuantity: 10})
	if err != nil {
		t.Fatal(err)
	}
	return db, repository, service
}

func seedOwnerAttempt(t *testing.T, db *gorm.DB, row testOwnerAttemptRow) {
	t.Helper()
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
}

func seedResourceBalance(t *testing.T, db *gorm.DB, organizationID string, resourceType orgresource.ResourceType, quantity int64) {
	t.Helper()
	repository, err := NewGormRepository(db, TransactionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := orgresource.NewService(repository, &mutableEligibilityVerifier{approval: orgresource.WelcomeGrantApproval{
		OrganizationID: organizationID, EvidenceID: "seed:" + organizationID, ApprovedAt: time.Now().UTC(),
	}}, allowWelcomeGrantAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	if quantity == 0 {
		if err := db.Create(&organizationResourceBucketRow{OrganizationID: organizationID, ResourceType: string(resourceType)}).Error; err != nil {
			t.Fatal(err)
		}
		return
	}
	// Welcome grant intentionally only mints one store period, so tests seed
	// other resource types through the persistence boundary with a complete
	// operation/event/audit fixture instead of exposing a generic grant API.
	now := time.Now().UTC()
	operationID := "seed-operation-" + organizationID
	operation := organizationResourceOperationRow{
		OrganizationID: organizationID, OperationID: operationID, OperationType: "test_seed_credit",
		RequestFingerprint: "test-seed", State: "succeeded", ImmutableResult: "{}", CompletedAt: &now,
	}
	if err := db.Create(&operation).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&organizationResourceBucketRow{OrganizationID: organizationID, ResourceType: string(resourceType), Available: quantity}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Omit("Operation").Create(&organizationResourceEventRow{
		EventID: "seed-event-" + organizationID, OrganizationID: organizationID, OperationID: operationID,
		ResourceType: string(resourceType), Quantity: quantity, AvailableDelta: quantity, Reason: "test_seed_credit",
		SourceType: "test_seed_credit", SourceIdentity: organizationID, BalanceAfter: quantity, AvailableAfter: quantity,
		GrossCredit: quantity, NetCredit: quantity,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Omit("Operation").Create(&organizationResourceAuditLogRow{
		OrganizationID: organizationID, OperationID: operationID, Action: "test_seed_credit", ActorID: "test", Payload: "{}",
	}).Error; err != nil {
		t.Fatal(err)
	}
	_ = service
}

func validRepositoryReserveInput() orgresource.ReserveInput {
	return orgresource.ReserveInput{
		OrganizationID: "org-a", OperationID: "operation-a", OwnerType: "listingkit_generation",
		OwnerAttemptID: "attempt-a", BusinessScope: "listing-kit:task-a", ResourceType: orgresource.ResourceAIPoint,
		Quantity: 1, ReservationPurpose: "generation", Principal: orgresource.Principal{ID: "listingkit-worker", Kind: orgresource.PrincipalTrustedProvisioning},
	}
}

func assertBucket(t *testing.T, db *gorm.DB, organizationID string, resourceType orgresource.ResourceType, available, reserved, consumed int64) {
	t.Helper()
	var row organizationResourceBucketRow
	if err := db.Where("organization_id = ? AND resource_type = ?", organizationID, resourceType).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Available != available || row.Reserved != reserved || row.Consumed != consumed {
		t.Fatalf("bucket = %#v, want available=%d reserved=%d consumed=%d", row, available, reserved, consumed)
	}
}
