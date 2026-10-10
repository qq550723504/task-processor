package storecenter_test

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"task-processor/internal/storecenter"
	"testing"
	"time"
)

func TestStoreLockReadRequiresBorrowedTransactionAndCurrentGrant(t *testing.T) {
	db := openStoreDB(t)
	access := &storeMemberAuthorizer{member: "membership-current"}
	repo, err := storecenter.NewMemberScopedStoreRepository(db, access)
	if err != nil {
		t.Fatal(err)
	}
	candidate := newPersistenceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Cockpit", "SG", "cockpit", time.Now().UTC())
	stored, _, err := repo.CreateOrReplay(context.Background(), "org-a", candidate)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.LockRead(context.Background(), "org-a", []string{stored.ID()}); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("unlocked root accepted: %v", err)
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		borrowed, err := storecenter.NewMemberScopedStoreRepository(tx, access)
		if err != nil {
			return err
		}
		if err := borrowed.LockRead(context.Background(), "org-a", []string{stored.ID()}); err != nil {
			return err
		}
		access.member = "other-membership"
		if err := borrowed.LockRead(context.Background(), "org-a", []string{stored.ID()}); !errors.Is(err, storecenter.ErrNotFound) {
			t.Fatalf("current grant bypassed: %v", err)
		}
		access.admin = true
		if err := borrowed.LockRead(context.Background(), "org-a", []string{uuid.NewString()}); !errors.Is(err, storecenter.ErrNotFound) {
			t.Fatalf("admin skipped Store existence: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
