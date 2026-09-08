package sourceaccountownershiprehearsal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRehearsalStorageUsesAValidationAndCleansOnlyItsOwnedRoot(t *testing.T) {
	storage, err := newRehearsalStorage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(storage.ProfileRoot, "101", "7", "profile.marker")); err != nil {
		t.Fatalf("synthetic profile marker: %v", err)
	}
	if err = storage.cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(storage.Root); !os.IsNotExist(err) {
		t.Fatalf("owned rehearsal root survived cleanup: %v", err)
	}
}

func TestRehearsalStorageCleanupRejectsAnotherTemporaryDirectory(t *testing.T) {
	other, err := os.MkdirTemp(os.TempDir(), "not-owned-by-source-account-b2-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(other) })
	marker := filepath.Join(other, "preserve")
	if err = os.WriteFile(marker, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = (rehearsalStorage{Root: other}).cleanup(); err == nil {
		t.Fatal("cleanup accepted a temporary directory it did not create")
	}
	if _, err = os.Stat(marker); err != nil {
		t.Fatalf("rejected cleanup changed the other directory: %v", err)
	}
}
