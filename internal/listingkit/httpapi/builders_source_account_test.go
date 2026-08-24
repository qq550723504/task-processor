package httpapi

import (
	"os"
	"strings"
	"testing"

	"gorm.io/gorm"

	"task-processor/internal/core/config"
)

func TestBuildSourceAccountRepositoryRetainsListingKitSchemaBootstrapPath(t *testing.T) {
	content, err := os.ReadFile("builders_source_account.go")
	if err != nil {
		t.Fatalf("read builders_source_account.go: %v", err)
	}
	source := string(content)
	for _, required := range []string{"buildRepositoryWithFallback", "newDBSourceAccountRepository"} {
		if !strings.Contains(source, required) {
			t.Fatalf("BuildSourceAccountRepository compatibility wrapper must retain %s", required)
		}
	}
}

func TestAutoMigrateSourceAccountRepositorySkipsWhenRuntimeMigrationIsDisabled(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_RUNTIME_AUTOMIGRATE", "false")

	if err := autoMigrateSourceAccountRepository(nil); err != nil {
		t.Fatalf("autoMigrateSourceAccountRepository() error = %v, want nil when disabled", err)
	}
}

func TestAutoMigrateSourceAccountRepositoryRejectsNilDBWhenEnabled(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_RUNTIME_AUTOMIGRATE", "true")

	if err := autoMigrateSourceAccountRepository((*gorm.DB)(nil)); err == nil {
		t.Fatal("autoMigrateSourceAccountRepository() error = nil, want nil database error when enabled")
	}
}

func TestBuildSourceAccountRepositoryWithoutDatabaseKeepsPublicPathAvailable(t *testing.T) {
	repository, closers, err := BuildSourceAccountRepository(&config.Config{}, nil)
	if err != nil {
		t.Fatalf("BuildSourceAccountRepository() error = %v", err)
	}
	if repository != nil || len(closers) != 0 {
		t.Fatalf("repository/closers = %v/%d, want nil/0 without database", repository, len(closers))
	}
}
