package storecenter_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"task-processor/internal/storecenter"
)

type storeMemberAuthorizer struct {
	member string
	admin  bool
}

func (a *storeMemberAuthorizer) AuthorizeStoreMember(_ context.Context, org string) (storecenter.StoreMemberAccess, error) {
	return storecenter.StoreMemberAccess{OrganizationID: org, ActorID: "subject-create", MemberID: a.member, Administrator: a.admin, CanWrite: true}, nil
}

func TestMemberStoreAccessFiltersBeforePaginationAndDoesNotInheritAfterRejoin(t *testing.T) {
	db := openStoreDB(t)
	access := &storeMemberAuthorizer{member: "membership-original"}
	repo, err := storecenter.NewMemberScopedStoreRepository(db, access)
	if err != nil {
		t.Fatal(err)
	}
	makeStore := func(name string) *storecenter.Store {
		candidate := newPersistenceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), name, "SG", name, time.Now().UTC())
		created, _, err := repo.CreateOrReplay(context.Background(), "org-a", candidate)
		if err != nil {
			t.Fatal(err)
		}
		return created
	}
	first := makeStore("first")
	makeStore("second")
	access.member = "another-membership"
	third := makeStore("third")
	page, err := repo.List(context.Background(), "org-a", storecenter.StoreListQuery{Page: 1, PageSize: 1})
	if err != nil || page.Total != 1 || len(page.Stores) != 1 || page.Stores[0].ID() != third.ID() {
		t.Fatalf("pagination leaked another member: %+v %v", page, err)
	}
	if _, err := repo.Get(context.Background(), "org-a", first.ID()); !errors.Is(err, storecenter.ErrNotFound) {
		t.Fatalf("unassigned Store was readable: %v", err)
	}
	access.member = "membership-rejoined"
	page, err = repo.List(context.Background(), "org-a", storecenter.StoreListQuery{Page: 1, PageSize: 10})
	if err != nil || page.Total != 0 {
		t.Fatalf("rejoined member inherited assignments: %+v %v", page, err)
	}
	access.admin = true
	page, err = repo.List(context.Background(), "org-a", storecenter.StoreListQuery{Page: 1, PageSize: 10})
	if err != nil || page.Total != 3 {
		t.Fatalf("admin cannot read organization Stores: %+v %v", page, err)
	}
	if err := repo.SetMemberGrant(context.Background(), storecenter.MemberStoreGrantCommand{OrganizationID: "org-a", StoreID: first.ID(), MemberID: "membership-rejoined", OperationID: uuid.NewString(), Active: true}); err != nil {
		t.Fatal(err)
	}
	access.admin = false
	if _, err := repo.Get(context.Background(), "org-a", first.ID()); err != nil {
		t.Fatal(err)
	}
	access.admin = true
	revoke := storecenter.MemberStoreGrantCommand{OrganizationID: "org-a", StoreID: first.ID(), MemberID: "membership-rejoined", OperationID: uuid.NewString(), ExpectedVersion: 1}
	if err := repo.SetMemberGrant(context.Background(), revoke); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetMemberGrant(context.Background(), revoke); err != nil {
		t.Fatalf("replay used changed grant version: %v", err)
	}
	changed := revoke
	changed.Active = true
	if err := repo.SetMemberGrant(context.Background(), changed); !errors.Is(err, storecenter.ErrAlreadyExists) {
		t.Fatalf("same operation accepted different grant payload: %v", err)
	}
	access.admin = false
	if _, err := repo.Get(context.Background(), "org-a", first.ID()); !errors.Is(err, storecenter.ErrNotFound) {
		t.Fatalf("revoked grant still readable: %v", err)
	}
	if _, _, err := repo.CreateOrReplay(context.Background(), "org-a", first); !errors.Is(err, storecenter.ErrNotFound) {
		t.Fatalf("create replay restored a revoked member grant: %v", err)
	}
	if err := first.TransitionTo(storecenter.RecordStatusActive, "subject-create", first.UpdatedAt().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", first, 1); !errors.Is(err, storecenter.ErrNotFound) {
		t.Fatalf("revoked assignment admitted a late Store mutation: %v", err)
	}
}

func TestMemberStoreCreateRollsBackWhenAssignmentCannotBeSaved(t *testing.T) {
	db := openStoreDB(t)
	access := &storeMemberAuthorizer{member: "membership-a"}
	repo, err := storecenter.NewMemberScopedStoreRepository(db, access)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TRIGGER reject_grant BEFORE INSERT ON workbench_store_member_grants BEGIN SELECT RAISE(ABORT,'synthetic grant failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	candidate := newPersistenceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "failure", "SG", "failure", time.Now().UTC())
	if _, _, err := repo.CreateOrReplay(context.Background(), "org-a", candidate); err == nil {
		t.Fatal("create succeeded without atomic member grant")
	}
	var count int64
	if err := db.Table("workbench_stores").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("unassigned Store persisted: %d %v", count, err)
	}
}
