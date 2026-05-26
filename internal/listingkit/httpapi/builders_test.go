package httpapi

import (
	"testing"

	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

func TestBuildListingKitTaskRepositoryFallsBackToInMemoryWithoutDatabase(t *testing.T) {
	t.Parallel()

	repo, closers, err := BuildListingKitTaskRepository(&config.Config{}, logrus.New())
	if err != nil {
		t.Fatalf("BuildListingKitTaskRepository() error = %v", err)
	}
	if repo == nil {
		t.Fatal("expected in-memory task repository")
	}
	if len(closers) != 0 {
		t.Fatalf("closers = %d, want 0", len(closers))
	}
}

func TestBuildListingAdminStoreRepositoryDisablesWithoutDatabase(t *testing.T) {
	t.Parallel()

	repo, closers, err := BuildListingAdminStoreRepository(&config.Config{}, logrus.New())
	if err != nil {
		t.Fatalf("BuildListingAdminStoreRepository() error = %v", err)
	}
	if repo != nil {
		t.Fatal("expected store admin repository to remain disabled without database")
	}
	if len(closers) != 0 {
		t.Fatalf("closers = %d, want 0", len(closers))
	}
}

func TestBuildListingSubscriptionRepositoryFallsBackToInMemoryWithoutDatabase(t *testing.T) {
	t.Parallel()

	repo, closers, err := BuildListingSubscriptionRepository(&config.Config{}, logrus.New())
	if err != nil {
		t.Fatalf("BuildListingSubscriptionRepository() error = %v", err)
	}
	if repo == nil {
		t.Fatal("expected in-memory listing subscription repository")
	}
	if len(closers) != 0 {
		t.Fatalf("closers = %d, want 0", len(closers))
	}
}

func TestBuildListingAdminImportTaskRepositoryDisablesWithoutDatabase(t *testing.T) {
	t.Parallel()

	repo, closers, err := BuildListingAdminImportTaskRepository(&config.Config{}, logrus.New())
	if err != nil {
		t.Fatalf("BuildListingAdminImportTaskRepository() error = %v", err)
	}
	if repo != nil {
		t.Fatal("expected import task admin repository to remain disabled without database")
	}
	if len(closers) != 0 {
		t.Fatalf("closers = %d, want 0", len(closers))
	}
}
