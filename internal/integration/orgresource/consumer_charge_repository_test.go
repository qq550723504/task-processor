package orgresourceadapter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"task-processor/internal/ledger/orgresource"
)

func TestConsumerChargeMemberReleaseReturnsNetToMemberAndCommitsOnce(t *testing.T) {
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&organizationResourceBucketRow{OrganizationID: "org-a", ResourceType: "data_row", Available: 10}).Error; err != nil {
		t.Fatal(err)
	}
	allocations, _ := NewGormMemberAllocationRepository(db, TransactionConfig{})
	_, err := allocations.Transfer(context.Background(), orgresource.MemberResourceTransfer{OrganizationID: "org-a", MemberID: "membership-a", ActorID: "admin", OperationID: "allocate", ResourceType: orgresource.ResourceDataRow, Action: orgresource.MemberResourceAllocate, Quantity: 10})
	if err != nil {
		t.Fatal(err)
	}
	repo, err := NewGormConsumerChargeRepository(db, TransactionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	owner := &chargeTestOwner{intents: map[orgresource.ConsumerChargeIdentity]orgresource.ConsumerChargeIntent{}, proofs: map[string]orgresource.ConsumerChargeProof{}}
	service, err := orgresource.NewConsumerChargeService(repo, map[orgresource.ResourceConsumer]orgresource.ConsumerChargeOwner{orgresource.ConsumerProductAcquisition: owner})
	if err != nil {
		t.Fatal(err)
	}
	intent := chargeTestIntent("acquisition-a", orgresource.FundingMember)
	owner.intents[intent.Identity] = intent
	first, err := service.Reserve(context.Background(), intent.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if first.State != orgresource.ReservationReserved {
		t.Fatalf("reservation: %#v", first)
	}
	if _, err := service.Reconcile(context.Background(), intent.Identity); !errors.Is(err, orgresource.ErrConsumerChargeUnknown) {
		t.Fatalf("unknown released reservation: %v", err)
	}
	if err := db.Create(&organizationResourceDebtRow{OrganizationID: "org-a", ResourceType: "data_row", Amount: 1}).Error; err != nil {
		t.Fatal(err)
	}
	owner.proofs[first.ReservationID] = orgresource.ConsumerChargeProof{Intent: intent, ReservationID: first.ReservationID, State: orgresource.ConsumerEffectFailed, EvidenceID: "fenced:no-publication"}
	released, err := service.Reconcile(context.Background(), intent.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if released.State != orgresource.ReservationReleased {
		t.Fatalf("release: %#v", released)
	}
	if released.BalanceAfter != 9 {
		t.Fatalf("receipt returned another funding pool or ignored debt: %d", released.BalanceAfter)
	}
	position, err := allocations.ReadPosition(context.Background(), "org-a", "membership-a", orgresource.ResourceDataRow)
	if err != nil {
		t.Fatal(err)
	}
	if position.Free != 9 || position.Reserved != 0 || position.Consumed != 0 {
		t.Fatalf("member debt release: %#v", position)
	}
	var bucket organizationResourceBucketRow
	if err := db.Take(&bucket).Error; err != nil {
		t.Fatal(err)
	}
	if bucket.Available != 0 || bucket.Allocated != 9 || bucket.Reserved != 0 {
		t.Fatalf("released into wrong pool: %#v", bucket)
	}
	intent = chargeTestIntent("acquisition-b", orgresource.FundingMember)
	owner.intents[intent.Identity] = intent
	second, err := service.Reserve(context.Background(), intent.Identity)
	if err != nil {
		t.Fatal(err)
	}
	owner.proofs[second.ReservationID] = orgresource.ConsumerChargeProof{Intent: intent, ReservationID: second.ReservationID, State: orgresource.ConsumerEffectSucceeded, EvidenceID: "catalog:version:7"}
	for n := 0; n < 2; n++ {
		if receipt, err := service.Reconcile(context.Background(), intent.Identity); err != nil || receipt.State != orgresource.ReservationCommitted {
			t.Fatalf("commit %d: %#v %v", n, receipt, err)
		} else if receipt.BalanceAfter != 8 {
			t.Fatalf("receipt failed to preserve original member balance: %d", receipt.BalanceAfter)
		}
	}
	position, err = allocations.ReadPosition(context.Background(), "org-a", "membership-a", orgresource.ResourceDataRow)
	if err != nil {
		t.Fatal(err)
	}
	if position.Free != 8 || position.Reserved != 0 || position.Consumed != 1 {
		t.Fatalf("double charged: %#v", position)
	}
	assertTableCount(t, db, "saas_organization_resource_reservations", 2)
}

func TestConsumerChargeRecoveryDoesNotStarveBehindTwentyFiveUnknown(t *testing.T) {
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&organizationResourceBucketRow{OrganizationID: "org-a", ResourceType: "data_row", Available: 30}).Error; err != nil {
		t.Fatal(err)
	}
	repo, _ := NewGormConsumerChargeRepository(db, TransactionConfig{})
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	repo.now = func() time.Time { return now }
	owner := &chargeTestOwner{intents: map[orgresource.ConsumerChargeIdentity]orgresource.ConsumerChargeIntent{}, proofs: map[string]orgresource.ConsumerChargeProof{}}
	service, _ := orgresource.NewConsumerChargeService(repo, map[orgresource.ResourceConsumer]orgresource.ConsumerChargeOwner{orgresource.ConsumerProductAcquisition: owner})
	var last orgresource.ConsumerChargeIdentity
	for n := 0; n < 30; n++ {
		intent := chargeTestIntent(fmt.Sprintf("acquisition-%02d", n), orgresource.FundingEnterprise)
		owner.intents[intent.Identity] = intent
		receipt, err := service.Reserve(context.Background(), intent.Identity)
		if err != nil {
			t.Fatal(err)
		}
		if n == 29 {
			last = intent.Identity
			owner.proofs[receipt.ReservationID] = orgresource.ConsumerChargeProof{Intent: intent, ReservationID: receipt.ReservationID, State: orgresource.ConsumerEffectSucceeded, EvidenceID: "catalog:done"}
		}
	}
	for n := 0; n < 2; n++ {
		if _, err := service.RecoverDue(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	receipt, err := repo.Read(context.Background(), last)
	if err != nil || receipt.State != orgresource.ReservationCommitted {
		t.Fatalf("later terminal reservation starved: %#v %v", receipt, err)
	}
	var bucket organizationResourceBucketRow
	if err := db.Take(&bucket).Error; err != nil {
		t.Fatal(err)
	}
	if bucket.Reserved != 29 || bucket.Consumed != 1 || bucket.Available != 0 {
		t.Fatalf("unknown work released: %#v", bucket)
	}
	if owner.intentReads != 30 {
		t.Fatalf("recovery reopened owner intents: %d", owner.intentReads)
	}
}

func TestConsumerChargeRejectsProofForDifferentMember(t *testing.T) {
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&organizationResourceBucketRow{OrganizationID: "org-a", ResourceType: "data_row", Available: 1}).Error; err != nil {
		t.Fatal(err)
	}
	repo, _ := NewGormConsumerChargeRepository(db, TransactionConfig{})
	intent := chargeTestIntent("acquisition", orgresource.FundingEnterprise)
	owner := &chargeTestOwner{intents: map[orgresource.ConsumerChargeIdentity]orgresource.ConsumerChargeIntent{intent.Identity: intent}, proofs: map[string]orgresource.ConsumerChargeProof{}}
	service, _ := orgresource.NewConsumerChargeService(repo, map[orgresource.ResourceConsumer]orgresource.ConsumerChargeOwner{orgresource.ConsumerProductAcquisition: owner})
	receipt, err := service.Reserve(context.Background(), intent.Identity)
	if err != nil {
		t.Fatal(err)
	}
	wrong := intent
	wrong.MemberID = "another-membership"
	owner.proofs[receipt.ReservationID] = orgresource.ConsumerChargeProof{Intent: wrong, ReservationID: receipt.ReservationID, State: orgresource.ConsumerEffectSucceeded, EvidenceID: "catalog"}
	if _, err := service.Reconcile(context.Background(), intent.Identity); !errors.Is(err, orgresource.ErrIdempotencyKeyConflict) {
		t.Fatalf("accepted unrelated proof: %v", err)
	}
	if receipt, err := repo.Read(context.Background(), intent.Identity); err != nil || receipt.State != orgresource.ReservationReserved {
		t.Fatalf("mismatch changed reservation: %#v %v", receipt, err)
	}
}

func TestConsumerChargeRecoveryOnlyClaimsRegisteredOwners(t *testing.T) {
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	for _, resource := range []string{"data_row", "store_renewal_period"} {
		if err := db.Create(&organizationResourceBucketRow{OrganizationID: "org-a", ResourceType: resource, Available: 1}).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo, _ := NewGormConsumerChargeRepository(db, TransactionConfig{})
	owner := &chargeTestOwner{intents: map[orgresource.ConsumerChargeIdentity]orgresource.ConsumerChargeIntent{}, proofs: map[string]orgresource.ConsumerChargeProof{}}
	product := chargeTestIntent("product", orgresource.FundingEnterprise)
	store := chargeTestIntent("store", orgresource.FundingEnterprise)
	store.Identity.Consumer = orgresource.ConsumerStoreService
	store.ResourceType = orgresource.ResourceStoreRenewalPeriod
	for _, intent := range []orgresource.ConsumerChargeIntent{product, store} {
		owner.intents[intent.Identity] = intent
		receipt, err := repo.Reserve(context.Background(), intent)
		if err != nil {
			t.Fatal(err)
		}
		owner.proofs[receipt.ReservationID] = orgresource.ConsumerChargeProof{Intent: intent, ReservationID: receipt.ReservationID, State: orgresource.ConsumerEffectSucceeded, EvidenceID: "owner:done"}
	}
	productService, _ := orgresource.NewConsumerChargeService(repo, map[orgresource.ResourceConsumer]orgresource.ConsumerChargeOwner{orgresource.ConsumerProductAcquisition: owner})
	if count, err := productService.RecoverDue(context.Background()); err != nil || count != 1 {
		t.Fatalf("product recovery: %d %v", count, err)
	}
	storeService, _ := orgresource.NewConsumerChargeService(repo, map[orgresource.ResourceConsumer]orgresource.ConsumerChargeOwner{orgresource.ConsumerStoreService: owner})
	if count, err := storeService.RecoverDue(context.Background()); err != nil || count != 1 {
		t.Fatalf("unregistered Store work was delayed by Product recovery: %d %v", count, err)
	}
}

func chargeTestIntent(operation string, funding orgresource.ResourceFunding) orgresource.ConsumerChargeIntent {
	return orgresource.ConsumerChargeIntent{Identity: orgresource.ConsumerChargeIdentity{OrganizationID: "org-a", Consumer: orgresource.ConsumerProductAcquisition, OperationID: operation}, ActorID: "user-a", MemberID: "membership-a", Funding: funding, ResourceType: orgresource.ResourceDataRow, Quantity: 1, Fingerprint: strings.Repeat("a", 64), BusinessScope: "1688:product-a"}
}

type chargeTestOwner struct {
	intents     map[orgresource.ConsumerChargeIdentity]orgresource.ConsumerChargeIntent
	proofs      map[string]orgresource.ConsumerChargeProof
	intentReads int
}

func (o *chargeTestOwner) ReadChargeIntent(_ context.Context, identity orgresource.ConsumerChargeIdentity) (orgresource.ConsumerChargeIntent, error) {
	o.intentReads++
	if intent, ok := o.intents[identity]; ok {
		return intent, nil
	}
	return orgresource.ConsumerChargeIntent{}, orgresource.ErrConsumerChargeUnknown
}
func (o *chargeTestOwner) ReadChargeProof(_ context.Context, receipt orgresource.ConsumerChargeReceipt) (orgresource.ConsumerChargeProof, error) {
	if proof, ok := o.proofs[receipt.ReservationID]; ok {
		return proof, nil
	}
	return orgresource.ConsumerChargeProof{Intent: receipt.Intent, ReservationID: receipt.ReservationID, State: orgresource.ConsumerEffectUnknown}, nil
}
