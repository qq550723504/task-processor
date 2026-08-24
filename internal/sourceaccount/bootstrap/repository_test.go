package bootstrap

import (
	"testing"

	"gorm.io/gorm"

	"task-processor/internal/core/config"
)

func TestAutoMigrateRepositorySkipsWhenRuntimeMigrationIsDisabled(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_RUNTIME_AUTOMIGRATE", "false")

	if err := autoMigrateRepository(nil); err != nil {
		t.Fatalf("autoMigrateRepository() error = %v, want nil when disabled", err)
	}
}

func TestAutoMigrateRepositoryRejectsNilDBWhenEnabled(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_RUNTIME_AUTOMIGRATE", "true")

	if err := autoMigrateRepository((*gorm.DB)(nil)); err == nil {
		t.Fatal("autoMigrateRepository() error = nil, want nil database error when enabled")
	}
}

func TestBuildRepositoryWithoutDatabaseKeepsPublicPathAvailable(t *testing.T) {
	repository, closers, err := BuildRepository(&config.Config{}, nil)
	if err != nil {
		t.Fatalf("BuildRepository() error = %v", err)
	}
	if repository != nil || len(closers) != 0 {
		t.Fatalf("repository/closers = %v/%d, want nil/0 without database", repository, len(closers))
	}
}
