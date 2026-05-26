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

func TestShouldDisableLegacyTenantResolverRequiresConfiguredDatabaseHost(t *testing.T) {
	t.Parallel()

	if !shouldDisableLegacyTenantResolver(nil) {
		t.Fatal("expected nil config to disable legacy tenant resolver")
	}
	if !shouldDisableLegacyTenantResolver(&config.Config{}) {
		t.Fatal("expected missing database config to disable legacy tenant resolver")
	}
	if !shouldDisableLegacyTenantResolver(&config.Config{Database: &config.DatabaseConfig{}}) {
		t.Fatal("expected blank database host to disable legacy tenant resolver")
	}
	if shouldDisableLegacyTenantResolver(&config.Config{Database: &config.DatabaseConfig{Host: "127.0.0.1"}}) {
		t.Fatal("expected configured database host to enable legacy tenant resolver probing")
	}
}

func TestLegacyTenantResolverDatabaseConfigsEnumeratesCandidateDatabases(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Database: &config.DatabaseConfig{
			Host:     "127.0.0.1",
			Port:     5432,
			Database: "app",
		},
	}

	candidates := legacyTenantResolverDatabaseConfigs(cfg)
	if len(candidates) != 2 {
		t.Fatalf("candidate count = %d, want 2", len(candidates))
	}
	if candidates[0].Database != "zitadel_auth" || candidates[1].Database != "zitadel" {
		t.Fatalf("candidate databases = [%s %s], want [zitadel_auth zitadel]", candidates[0].Database, candidates[1].Database)
	}
	if candidates[0].Host != cfg.Database.Host || candidates[1].Port != cfg.Database.Port {
		t.Fatal("expected candidate configs to preserve base connection settings")
	}
}
