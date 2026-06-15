package httpapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildersFamiliesOwnSeparatedResponsibilities(t *testing.T) {
	t.Parallel()

	dir := "."

	assertNoFile(t, filepath.Join(dir, "builders.go"))

	schemaFile := readFileContent(t, filepath.Join(dir, "builders_repository_schema.go"))
	repositoriesFile := readFileContent(t, filepath.Join(dir, "builders_repositories.go"))
	dbRepositoriesFile := readFileContent(t, filepath.Join(dir, "builders_db_repositories.go"))
	imageStoreFile := readFileContent(t, filepath.Join(dir, "builders_image_store.go"))
	legacyTenantFile := readFileContent(t, filepath.Join(dir, "builders_legacy_tenant.go"))
	recoveryFile := readFileContent(t, filepath.Join(dir, "builders_recovery.go"))

	assertContainsAll(t, schemaFile,
		"type repositorySchemaBootstrapper struct",
		"func ensureListingKitRepositorySchema",
		"func runListingKitRepositoryAutoMigrations",
	)
	assertNotContainsAny(t, schemaFile,
		"func BuildImageUploadStore",
		"func ConfigureLegacyTenantResolver",
		"func BuildListingKitTaskRecoverySweepInterval",
		"func newDBListingKitTaskRepository",
	)

	assertContainsAll(t, repositoriesFile,
		"func buildRepositoryWithFallback",
		"func BuildListingKitTaskRepository",
		"func BuildListingAdminStoreRepository",
		"func BuildSheinResolutionCacheStore",
	)
	assertNotContainsAny(t, repositoriesFile,
		"func ensureListingKitRepositorySchema",
		"func BuildImageUploadStore",
		"func ConfigureLegacyTenantResolver",
		"func newDBListingKitTaskRepository",
	)

	assertContainsAll(t, dbRepositoriesFile,
		"func newDBListingKitTaskRepository",
		"func newDBListingAdminStoreRepository",
		"func newDBListingSubscriptionRepository",
	)
	assertNotContainsAny(t, dbRepositoriesFile,
		"func BuildListingKitTaskRepository",
		"func BuildImageUploadStore",
		"func ConfigureLegacyTenantResolver",
		"func BuildListingKitTaskRecoverySweepInterval",
	)

	assertContainsAll(t, imageStoreFile,
		"func BuildSheinPricingPolicy",
		"func BuildImageUploadStore",
		"func buildS3ImageUploadStore",
	)
	assertNotContainsAny(t, imageStoreFile,
		"func BuildListingKitTaskRepository",
		"func ConfigureLegacyTenantResolver",
		"func BuildListingKitTaskRecoverySweepInterval",
	)

	assertContainsAll(t, legacyTenantFile,
		"func ConfigureLegacyTenantResolver",
		"func shouldDisableLegacyTenantResolver",
		"func legacyTenantMetadataTableExists",
	)
	assertNotContainsAny(t, legacyTenantFile,
		"func BuildImageUploadStore",
		"func BuildListingKitTaskRecoverySweepInterval",
		"func newDBListingKitTaskRepository",
	)

	assertContainsAll(t, recoveryFile,
		"const (",
		"func BuildListingKitTaskRecoverySweepInterval",
		"func BuildListingKitTaskRecoverySweepLimit",
	)
	assertNotContainsAny(t, recoveryFile,
		"func BuildImageUploadStore",
		"func ConfigureLegacyTenantResolver",
		"func newDBListingKitTaskRepository",
	)
}

func assertNoFile(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected %s to be removed", path)
	}
}

func readFileContent(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func assertContainsAll(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected content to contain %q", snippet)
		}
	}
}

func assertNotContainsAny(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected content to exclude %q", snippet)
		}
	}
}
