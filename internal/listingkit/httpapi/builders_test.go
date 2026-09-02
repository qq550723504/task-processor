package httpapi

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	"task-processor/internal/core/config"
	listingkitstore "task-processor/internal/listingkit/store"
)

func TestShouldAutoMigrateListingKitRuntimeDefaultsTrue(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_RUNTIME_AUTOMIGRATE", "")

	if !shouldAutoMigrateListingKitRuntime() {
		t.Fatal("expected listingkit runtime auto-migrate to default to true")
	}
}

func TestShouldAutoMigrateListingKitRuntimeHonorsFalse(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_RUNTIME_AUTOMIGRATE", "false")

	if shouldAutoMigrateListingKitRuntime() {
		t.Fatal("expected listingkit runtime auto-migrate to honor false")
	}
}

func TestAutoMigrateListingKitRuntimeSchemaRejectsNilDB(t *testing.T) {
	t.Parallel()

	err := AutoMigrateListingKitRuntimeSchema(nil)
	if err == nil {
		t.Fatal("expected nil db to fail")
	}
}

func TestAutoMigrateListingKitRuntimeSchemaCreatesAIInvocationsTable(t *testing.T) {
	db := openListingKitBuilderTestDB(t)

	if err := AutoMigrateListingKitRuntimeSchema(db); err != nil {
		t.Fatalf("AutoMigrateListingKitRuntimeSchema() error = %v", err)
	}
	if !db.Migrator().HasTable("ai_invocations") {
		t.Fatal("expected ai_invocations table to be created")
	}
}

func openListingKitBuilderTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	for _, table := range []string{
		"listing_store",
		"listing_product_import_task",
		"listing_filter_rule",
		"listing_profit_rule",
		"listing_pricing_rule",
		"listing_operation_strategy",
		"listing_sensitive_word",
		"listing_product_import_mapping",
		"listing_category",
		"listing_product_data",
	} {
		if err := db.Exec("CREATE TABLE " + table + " (id integer)").Error; err != nil {
			t.Fatalf("create legacy %s table: %v", table, err)
		}
	}
	return db
}

func TestRepositorySchemaBootstrapperRunsMigrationOncePerDatabase(t *testing.T) {
	t.Parallel()

	bootstrapper := newRepositorySchemaBootstrapper()
	cfg := &config.DatabaseConfig{
		Host:     "127.0.0.1",
		Port:     5432,
		User:     "tester",
		Database: "listingkit",
	}

	migrationRuns := 0
	runMigration := func() error {
		migrationRuns++
		return nil
	}

	if err := bootstrapper.ensure(cfg, runMigration); err != nil {
		t.Fatalf("first ensure() error = %v", err)
	}
	if err := bootstrapper.ensure(cfg, runMigration); err != nil {
		t.Fatalf("second ensure() error = %v", err)
	}

	if migrationRuns != 1 {
		t.Fatalf("migration runs = %d, want 1", migrationRuns)
	}
}

func TestBuildPersistentRepositoriesRejectsMissingDatabaseConfiguration(t *testing.T) {
	t.Parallel()

	for _, database := range []*config.DatabaseConfig{nil, {}} {
		if repositories, closer, err := BuildPersistentRepositories(database, nil); err == nil || closer != nil || repositories.Core.Task != nil {
			t.Fatalf("BuildPersistentRepositories(%#v) = %#v/%T/%v, want fail closed", database, repositories, closer, err)
		}
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

func TestLocalImageUploadRootDirUsesListingKitOwnedRoot(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		ListingKit: config.ListingKitConfig{ImageUpload: config.ListingKitImageUploadConfig{
			Local: config.ListingKitImageUploadLocalConfig{RootDir: "listingkit-owned-root"},
		}},
	}

	got := localImageUploadRootDir(cfg)
	want := cfg.ListingKit.ImageUpload.Local.RootDir
	if got != want {
		t.Fatalf("root dir = %q, want %q", got, want)
	}
}

func TestAutoMigrateSheinSyncRepositoryCreatesSDSCostGroupTable(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := listingkitstore.AutoMigrateSheinSyncRepository(db); err != nil {
		t.Fatalf("AutoMigrateSheinSyncRepository() error = %v", err)
	}

	if !db.Migrator().HasTable("listingkit_shein_sds_cost_groups") {
		t.Fatal("expected listingkit_shein_sds_cost_groups table to be created")
	}
}
