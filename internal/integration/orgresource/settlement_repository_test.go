package orgresourceadapter

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"task-processor/internal/ledger/orgresource"
)

func TestTerminalOwnerSuccessCommitsReservation(t *testing.T) {
	db, ownerStore, repository, reservationService, settlementService := newSettlementTestServices(t)
	seedResourceBalance(t, db, "org-a", orgresource.ResourceAIPoint, 2)
	seedOwnerAttempt(t, db, testOwnerAttemptRow{
		OrganizationID: "org-a", OwnerType: "listingkit_generation", AttemptID: "attempt-a",
		BusinessScope: "listing-kit:task-a", State: string(orgresource.OwnerAttemptNotStarted),
	})
	reserved, err := reservationService.Reserve(context.Background(), validRepositoryReserveInput())
	if err != nil {
		t.Fatal(err)
	}
	setOwnerTerminal(t, db, orgresource.OwnerAttemptSucceededTerminal, "provider-receipt-a")

	settled, err := settlementService.Settle(context.Background(), settlementInput(reserved.Snapshot.ReservationID, "settle-a"))
	if err != nil {
		t.Fatal(err)
	}
	if settled.Snapshot.Decision != orgresource.SettlementCommit || settled.Snapshot.AvailableAfter != "1" || settled.Snapshot.ReservedAfter != "0" || settled.Snapshot.ConsumedAfter != "1" {
		t.Fatalf("settled = %#v", settled)
	}
	assertBucket(t, db, "org-a", orgresource.ResourceAIPoint, 1, 0, 1)
	assertDebt(t, db, "org-a", orgresource.ResourceAIPoint, 0)
	_ = ownerStore
	_ = repository
}

func TestTerminalOwnerFailureReleasesThroughDebtFirst(t *testing.T) {
	db, _, _, reservationService, settlementService := newSettlementTestServices(t)
	seedResourceBalance(t, db, "org-a", orgresource.ResourceAIPoint, 2)
	seedOwnerAttempt(t, db, testOwnerAttemptRow{
		OrganizationID: "org-a", OwnerType: "listingkit_generation", AttemptID: "attempt-a",
		BusinessScope: "listing-kit:task-a", State: string(orgresource.OwnerAttemptNotStarted),
	})
	reserved, err := reservationService.Reserve(context.Background(), validRepositoryReserveInput())
	if err != nil {
		t.Fatal(err)
	}
	seedDebt(t, db, "org-a", orgresource.ResourceAIPoint, 1)
	setOwnerTerminal(t, db, orgresource.OwnerAttemptFailedTerminal, "provider-rejected-a")

	settled, err := settlementService.Settle(context.Background(), settlementInput(reserved.Snapshot.ReservationID, "settle-release"))
	if err != nil {
		t.Fatal(err)
	}
	if settled.Snapshot.Decision != orgresource.SettlementRelease || settled.Snapshot.GrossCredit != "1" || settled.Snapshot.DebtRepaid != "1" || settled.Snapshot.NetCredit != "0" {
		t.Fatalf("settled = %#v", settled)
	}
	assertBucket(t, db, "org-a", orgresource.ResourceAIPoint, 1, 0, 0)
	assertDebt(t, db, "org-a", orgresource.ResourceAIPoint, 0)
}

func TestSettlementRejectsNonTerminalOwnerWithoutMutation(t *testing.T) {
	for _, state := range []orgresource.OwnerAttemptState{orgresource.OwnerAttemptNotStarted, orgresource.OwnerAttemptProcessing, orgresource.OwnerAttemptOutcomeUnknown} {
		t.Run(string(state), func(t *testing.T) {
			db, _, _, reservationService, settlementService := newSettlementTestServices(t)
			seedResourceBalance(t, db, "org-a", orgresource.ResourceAIPoint, 1)
			seedOwnerAttempt(t, db, testOwnerAttemptRow{
				OrganizationID: "org-a", OwnerType: "listingkit_generation", AttemptID: "attempt-a",
				BusinessScope: "listing-kit:task-a", State: string(orgresource.OwnerAttemptNotStarted),
			})
			reserved, err := reservationService.Reserve(context.Background(), validRepositoryReserveInput())
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&testOwnerAttemptRow{}).Where("organization_id = ?", "org-a").Update("state", state).Error; err != nil {
				t.Fatal(err)
			}
			_, err = settlementService.Settle(context.Background(), settlementInput(reserved.Snapshot.ReservationID, "settle-a"))
			if !errors.Is(err, orgresource.ErrOwnerNotTerminal) {
				t.Fatalf("Settle() error = %v, want ErrOwnerNotTerminal", err)
			}
			assertBucket(t, db, "org-a", orgresource.ResourceAIPoint, 0, 1, 0)
			assertReservationState(t, db, reserved.Snapshot.ReservationID, orgresource.ReservationReserved)
		})
	}
}

func TestSettlementReplaysBeforeOwnerProofChangesAndRecoversLostCommit(t *testing.T) {
	db, ownerStore, repository, reservationService, settlementService := newSettlementTestServices(t)
	seedResourceBalance(t, db, "org-a", orgresource.ResourceAIPoint, 1)
	seedOwnerAttempt(t, db, testOwnerAttemptRow{
		OrganizationID: "org-a", OwnerType: "listingkit_generation", AttemptID: "attempt-a",
		BusinessScope: "listing-kit:task-a", State: string(orgresource.OwnerAttemptNotStarted),
	})
	reserved, err := reservationService.Reserve(context.Background(), validRepositoryReserveInput())
	if err != nil {
		t.Fatal(err)
	}
	setOwnerTerminal(t, db, orgresource.OwnerAttemptSucceededTerminal, "provider-receipt-a")
	repository.afterCommit = func() error { return errSyntheticCommitResponseLoss }
	first, err := settlementService.Settle(context.Background(), settlementInput(reserved.Snapshot.ReservationID, "settle-a"))
	if err != nil {
		t.Fatal(err)
	}
	if !first.Replayed {
		t.Fatalf("lost commit result = %#v", first)
	}
	repository.afterCommit = nil
	ownerStore.terminalErr = errors.New("owner proof unavailable")
	replayed, err := settlementService.Settle(context.Background(), settlementInput(reserved.Snapshot.ReservationID, "settle-b"))
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Replayed || replayed.Snapshot != first.Snapshot {
		t.Fatalf("first=%#v replayed=%#v", first, replayed)
	}
	assertBucket(t, db, "org-a", orgresource.ResourceAIPoint, 0, 0, 1)
}

func TestSettlementAuditFailureRollsBackReleaseAndDebtRepayment(t *testing.T) {
	db, _, _, reservationService, settlementService := newSettlementTestServices(t)
	seedResourceBalance(t, db, "org-a", orgresource.ResourceAIPoint, 1)
	seedOwnerAttempt(t, db, testOwnerAttemptRow{
		OrganizationID: "org-a", OwnerType: "listingkit_generation", AttemptID: "attempt-a",
		BusinessScope: "listing-kit:task-a", State: string(orgresource.OwnerAttemptNotStarted),
	})
	reserved, err := reservationService.Reserve(context.Background(), validRepositoryReserveInput())
	if err != nil {
		t.Fatal(err)
	}
	seedDebt(t, db, "org-a", orgresource.ResourceAIPoint, 1)
	setOwnerTerminal(t, db, orgresource.OwnerAttemptFailedTerminal, "provider-rejected-a")
	if err := db.Exec(`CREATE TRIGGER fail_settlement_audit
		BEFORE INSERT ON saas_organization_resource_audit_logs
		WHEN NEW.action = 'settle_reservation:release'
		BEGIN SELECT RAISE(ABORT, 'audit unavailable'); END`).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := settlementService.Settle(context.Background(), settlementInput(reserved.Snapshot.ReservationID, "settle-a")); err == nil {
		t.Fatal("Settle() error = nil, want audit failure")
	}
	assertBucket(t, db, "org-a", orgresource.ResourceAIPoint, 0, 1, 0)
	assertDebt(t, db, "org-a", orgresource.ResourceAIPoint, 1)
	assertReservationState(t, db, reserved.Snapshot.ReservationID, orgresource.ReservationReserved)
}

func settlementInput(reservationID, operationID string) orgresource.SettlementInput {
	return orgresource.SettlementInput{
		OrganizationID: "org-a", OperationID: operationID, ReservationID: reservationID,
		Principal: orgresource.Principal{ID: "owner-reconciler", Kind: orgresource.PrincipalTrustedProvisioning},
	}
}

type allowSettlementAuthorizer struct{}

func (allowSettlementAuthorizer) AuthorizeSettlement(context.Context, orgresource.Principal) error {
	return nil
}

func newSettlementTestServices(t *testing.T) (*gorm.DB, *testOwnerStore, *GormReservationRepository, *orgresource.ReservationService, *orgresource.SettlementService) {
	t.Helper()
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&testOwnerAttemptRow{}); err != nil {
		t.Fatal(err)
	}
	ownerStore := &testOwnerStore{}
	repository, err := NewGormReservationRepository(db, TransactionConfig{}, map[string]TransactionalReservationOwnerStore{
		"listingkit_generation": ownerStore,
	})
	if err != nil {
		t.Fatal(err)
	}
	reservationService, err := orgresource.NewReservationService(repository, allowReservationAuthorizer{maxQuantity: 10})
	if err != nil {
		t.Fatal(err)
	}
	settlementService, err := orgresource.NewSettlementService(repository, allowSettlementAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	return db, ownerStore, repository, reservationService, settlementService
}

func setOwnerTerminal(t *testing.T, db *gorm.DB, state orgresource.OwnerAttemptState, evidenceID string) {
	t.Helper()
	updated := db.Model(&testOwnerAttemptRow{}).
		Where("organization_id = ? AND owner_type = ? AND attempt_id = ?", "org-a", "listingkit_generation", "attempt-a").
		Updates(map[string]any{"state": state, "terminal_evidence_id": evidenceID})
	if updated.Error != nil {
		t.Fatal(updated.Error)
	}
	if updated.RowsAffected != 1 {
		t.Fatalf("terminal owner rows = %d, want 1", updated.RowsAffected)
	}
}

func seedDebt(t *testing.T, db *gorm.DB, organizationID string, resourceType orgresource.ResourceType, amount int64) {
	t.Helper()
	if err := db.Create(&organizationResourceDebtRow{OrganizationID: organizationID, ResourceType: string(resourceType), Amount: amount}).Error; err != nil {
		t.Fatal(err)
	}
}

func assertDebt(t *testing.T, db *gorm.DB, organizationID string, resourceType orgresource.ResourceType, want int64) {
	t.Helper()
	var row organizationResourceDebtRow
	err := db.Where("organization_id = ? AND resource_type = ?", organizationID, resourceType).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if want != 0 {
			t.Fatalf("debt missing, want %d", want)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if row.Amount != want {
		t.Fatalf("debt = %d, want %d", row.Amount, want)
	}
}

func assertReservationState(t *testing.T, db *gorm.DB, reservationID string, want orgresource.ReservationState) {
	t.Helper()
	var row organizationResourceReservationRow
	if err := db.Where("organization_id = ? AND reservation_id = ?", "org-a", reservationID).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != string(want) {
		t.Fatalf("reservation state = %q, want %q", row.State, want)
	}
}
