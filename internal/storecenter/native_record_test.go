package storecenter_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"task-processor/internal/storecenter"
)

func nativeCandidate(t *testing.T, key, actor string) *storecenter.Store {
	t.Helper()
	value, err := storecenter.NewStore(storecenter.CreateStoreInput{ID: uuid.NewString(), OrganizationID: "org-a", ActorSubject: actor, Name: "Native store", Platform: "shein", Region: "SG", ExternalStoreID: "native-shop", CreateIdempotencyKey: key, OccurredAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestNativeStoreCreateUsesBusinessFingerprintAndOriginalActor(t *testing.T) {
	db := openStoreDB(t)
	if err := storecenter.AutoMigrateAuditRepository(db); err != nil {
		t.Fatal(err)
	}
	access := &storeMemberAuthorizer{member: "original-membership"}
	repo, _ := storecenter.NewMemberScopedStoreRepository(db, access)
	key := uuid.NewString()
	first, replayed, err := repo.CreateOrReplay(context.Background(), "org-a", nativeCandidate(t, key, "subject-create"))
	if err != nil || replayed {
		t.Fatalf("first=%+v replayed=%v err=%v", first, replayed, err)
	}
	if first.RecordStatus() != storecenter.RecordStatusActive || first.ServiceStatus() != storecenter.ServiceStatusPendingActivation {
		t.Fatalf("created state=%+v", first.Snapshot())
	}
	access.admin = true
	replay, replayed, err := repo.CreateOrReplay(context.Background(), "org-a", nativeCandidate(t, key, "other-admin"))
	if err != nil || !replayed || replay.ID() != first.ID() || replay.CreatedBy() != first.CreatedBy() {
		t.Fatalf("replay=%+v replayed=%v err=%v", replay, replayed, err)
	}
	audit, _ := storecenter.NewGormAuditRepository(db)
	event, err := audit.Get(context.Background(), "org-a", key, storecenter.AuditActionStoreCreated)
	if err != nil || event.ActorSubject != "subject-create" || event.StoreID != first.ID() {
		t.Fatalf("audit=%+v err=%v", event, err)
	}
	if err := repo.SetMemberGrant(context.Background(), storecenter.MemberStoreGrantCommand{OrganizationID: "org-a", StoreID: first.ID(), MemberID: access.member, OperationID: uuid.NewString(), ExpectedVersion: 1}); err != nil {
		t.Fatal(err)
	}
	access.admin = false
	if _, _, err := repo.CreateOrReplay(context.Background(), "org-a", nativeCandidate(t, key, "subject-create")); !errors.Is(err, storecenter.ErrNotFound) {
		t.Fatalf("revoked grant replay=%v", err)
	}
}

func TestNativeStoreCreateRollsBackStoreGrantAndReceiptWhenAuditFails(t *testing.T) {
	db := openStoreDB(t)
	if err := storecenter.AutoMigrateAuditRepository(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TRIGGER fail_native_create_audit BEFORE INSERT ON workbench_store_audit_logs BEGIN SELECT RAISE(ABORT,'synthetic audit failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	repo, _ := storecenter.NewMemberScopedStoreRepository(db, &storeMemberAuthorizer{member: "membership-a"})
	if _, _, err := repo.CreateOrReplay(context.Background(), "org-a", nativeCandidate(t, uuid.NewString(), "subject-create")); err == nil {
		t.Fatal("audit failure accepted")
	}
	for _, table := range []string{"workbench_stores", "workbench_store_member_grants", "workbench_store_member_grant_operations"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("%s count=%d err=%v", table, count, err)
		}
	}
}
