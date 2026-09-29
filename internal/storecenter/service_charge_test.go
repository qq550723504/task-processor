package storecenter_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"task-processor/internal/storecenter"
)

func TestStoreServiceProofAndDatesCommitOnceAndRevocationFencesPendingCommand(t *testing.T) {
	db := openStoreDB(t)
	access := &storeMemberAuthorizer{member: "membership-a"}
	repo, err := storecenter.NewMemberScopedStoreRepository(db, access)
	if err != nil {
		t.Fatal(err)
	}
	candidate := newPersistenceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "service", "SG", "service", time.Now().UTC().Add(-time.Minute))
	created, _, err := repo.CreateOrReplay(context.Background(), "org-a", candidate)
	if err != nil {
		t.Fatal(err)
	}
	if err := created.TransitionTo(storecenter.RecordStatusActive, "subject-create", created.UpdatedAt().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", created, 1); err != nil {
		t.Fatal(err)
	}
	intent := storecenter.ServiceChargeIntent{MemberID: access.member, Funding: "member_allocated", Execution: storecenter.ServiceExecution{OrganizationID: "org-a", StoreID: created.ID(), OperationID: uuid.NewString(), Command: storecenter.ServiceCommandActivate, Quantity: 1, MaxQuantity: 1, ExpectedStoreVersion: 2, ActorSubject: "subject-create", OccurredAt: time.Now().UTC(), RequestFingerprint: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ConnectionStatus: storecenter.ConnectionStatusConnected}}
	if _, err := repo.AdmitServiceCharge(context.Background(), intent); err != nil {
		t.Fatal(err)
	}
	reservation := uuid.NewString()
	if err := repo.BindServiceCharge(context.Background(), intent, reservation); err != nil {
		t.Fatal(err)
	}
	proof, err := repo.ApplyServiceCharge(context.Background(), intent, storecenter.ConnectionStatusConnected)
	if err != nil || proof.State != "succeeded" || proof.Snapshot.StoreVersion != 3 {
		t.Fatalf("service effect/proof: %+v %v", proof, err)
	}
	firstExpiry := *proof.Snapshot.ServiceState.ExpiresAt
	replay, err := repo.ApplyServiceCharge(context.Background(), intent, storecenter.ConnectionStatusConnected)
	if err != nil || replay.EvidenceID != proof.EvidenceID || !replay.Snapshot.ServiceState.ExpiresAt.Equal(firstExpiry) {
		t.Fatalf("same command extended dates twice: %+v %v", replay, err)
	}
	intent.Execution.OperationID = uuid.NewString()
	intent.Execution.Command = storecenter.ServiceCommandRenew
	intent.Execution.Quantity = 2
	intent.Execution.MaxQuantity = 12
	intent.Execution.ExpectedStoreVersion = 3
	intent.Execution.RequestFingerprint = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if _, err := repo.AdmitServiceCharge(context.Background(), intent); err != nil {
		t.Fatal(err)
	}
	if err := repo.BindServiceCharge(context.Background(), intent, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	access.admin = true
	if err := repo.SetMemberGrant(context.Background(), storecenter.MemberStoreGrantCommand{OrganizationID: "org-a", StoreID: created.ID(), MemberID: access.member, OperationID: uuid.NewString(), ExpectedVersion: 1}); err != nil {
		t.Fatal(err)
	}
	access.admin = false
	failed, err := repo.ApplyServiceCharge(context.Background(), intent, "")
	if err != nil || failed.State != "failed_fenced" || failed.ReservationID == "" {
		t.Fatalf("revocation has no definitive original-reservation proof: %+v %v", failed, err)
	}
	access.admin = true
	if err := repo.SetMemberGrant(context.Background(), storecenter.MemberStoreGrantCommand{OrganizationID: "org-a", StoreID: created.ID(), MemberID: access.member, OperationID: uuid.NewString(), ExpectedVersion: 2, Active: true}); err != nil {
		t.Fatal(err)
	}
	access.admin = false
	late, err := repo.ApplyServiceCharge(context.Background(), intent, "")
	if err != nil || late.State != "failed_fenced" || late.EvidenceID != failed.EvidenceID {
		t.Fatalf("late command revived a failed reservation: %+v %v", late, err)
	}
	actual, err := repo.Get(context.Background(), "org-a", created.ID())
	if err != nil || actual.Version() != 3 || !actual.Snapshot().ServiceExpiresAt.Equal(firstExpiry) {
		t.Fatalf("revoked command changed dates: %+v %v", actual, err)
	}
}
