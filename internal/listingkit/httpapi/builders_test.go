package httpapi

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
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

func TestShouldUseS3ImageUploadStoreMatchesConfiguredProvider(t *testing.T) {
	t.Parallel()

	if shouldUseS3ImageUploadStore(nil) {
		t.Fatal("expected nil config to skip s3 image upload store")
	}
	if shouldUseS3ImageUploadStore(&config.Config{}) {
		t.Fatal("expected blank provider to skip s3 image upload store")
	}
	if !shouldUseS3ImageUploadStore(&config.Config{ProductImage: config.ProductImageConfig{Publisher: config.ProductImagePublisherConfig{Provider: " S3 "}}}) {
		t.Fatal("expected s3 provider to enable s3 image upload store")
	}
}

func TestLocalImageUploadRootDirUsesPublisherOutputDir(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		ProductImage: config.ProductImageConfig{
			Publisher: config.ProductImagePublisherConfig{
				OutputDir: filepath.Join("tmp", "publisher"),
			},
		},
	}

	got := localImageUploadRootDir(cfg)
	want := filepath.Join(cfg.ProductImage.Publisher.OutputDir, "listingkit-inputs")
	if got != want {
		t.Fatalf("root dir = %q, want %q", got, want)
	}
}

func TestAutoMigrateListingKitTaskRepositoryCreatesSDSBaselineCacheTable(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := autoMigrateListingKitTaskRepository(db); err != nil {
		t.Fatalf("autoMigrateListingKitTaskRepository() error = %v", err)
	}

	if !db.Migrator().HasTable("listing_kit_sds_baseline_cache") {
		t.Fatal("expected listing_kit_sds_baseline_cache table to be created")
	}
}
