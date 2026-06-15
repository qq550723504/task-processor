package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSheinSyncRepositoryFilesOwnSeparatedFamilies(t *testing.T) {
	t.Parallel()

	dir := "."

	rootFile := readSheinSyncRepoFileContent(t, filepath.Join(dir, "shein_sync_repo.go"))
	productFile := readSheinSyncRepoFileContent(t, filepath.Join(dir, "shein_sync_repo_products.go"))
	jobFile := readSheinSyncRepoFileContent(t, filepath.Join(dir, "shein_sync_repo_jobs.go"))
	enrollmentFile := readSheinSyncRepoFileContent(t, filepath.Join(dir, "shein_sync_repo_enrollment.go"))
	filterFile := readSheinSyncRepoFileContent(t, filepath.Join(dir, "shein_sync_repo_filters.go"))

	assertSheinSyncRepoContainsAll(t, rootFile,
		"type GormSheinSyncRepository struct {",
		"func NewSheinSyncRepository(db *gorm.DB) listingkit.SheinSyncRepository {",
		"func AutoMigrateSheinSyncRepository(db *gorm.DB) error {",
	)
	assertSheinSyncRepoNotContainsAny(t, rootFile,
		"func (r *GormSheinSyncRepository) UpsertSyncedProducts(",
		"func (r *GormSheinSyncRepository) SaveSyncJob(",
		"func (r *GormSheinSyncRepository) SaveCandidates(",
		"func normalizeSheinSyncPage(",
	)

	assertSheinSyncRepoContainsAll(t, productFile,
		"func (r *GormSheinSyncRepository) UpsertSyncedProducts(",
		"func (r *GormSheinSyncRepository) ListSyncedProducts(",
		"func (r *GormSheinSyncRepository) UpdateManualCostPrice(",
		"func sheinSyncedProductAssignments(",
	)
	assertSheinSyncRepoNotContainsAny(t, productFile,
		"func (r *GormSheinSyncRepository) SaveSyncJob(",
		"func (r *GormSheinSyncRepository) SaveCandidates(",
		"func normalizeSheinSyncPage(",
	)

	assertSheinSyncRepoContainsAll(t, jobFile,
		"func (r *GormSheinSyncRepository) SaveSyncJob(",
		"func (r *GormSheinSyncRepository) ListSyncJobs(",
	)
	assertSheinSyncRepoNotContainsAny(t, jobFile,
		"func (r *GormSheinSyncRepository) UpsertSyncedProducts(",
		"func (r *GormSheinSyncRepository) SaveCandidates(",
		"func normalizeSheinSyncPage(",
	)

	assertSheinSyncRepoContainsAll(t, enrollmentFile,
		"func (r *GormSheinSyncRepository) SaveCandidates(",
		"func (r *GormSheinSyncRepository) ListCandidates(",
		"func (r *GormSheinSyncRepository) CreateEnrollmentRun(",
		"func (r *GormSheinSyncRepository) SaveEnrollmentItems(",
	)
	assertSheinSyncRepoNotContainsAny(t, enrollmentFile,
		"func (r *GormSheinSyncRepository) UpsertSyncedProducts(",
		"func (r *GormSheinSyncRepository) SaveSyncJob(",
		"func normalizeSheinSyncPage(",
	)

	assertSheinSyncRepoContainsAll(t, filterFile,
		"func normalizeSheinSyncPage(",
		"func applySheinSyncedProductFilters(",
		"func applySheinSyncJobFilters(",
		"func sheinCandidateKey(",
	)
	assertSheinSyncRepoNotContainsAny(t, filterFile,
		"func (r *GormSheinSyncRepository) UpsertSyncedProducts(",
		"func (r *GormSheinSyncRepository) SaveSyncJob(",
		"func (r *GormSheinSyncRepository) SaveCandidates(",
	)
}

func readSheinSyncRepoFileContent(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func assertSheinSyncRepoContainsAll(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected content to contain %q", snippet)
		}
	}
}

func assertSheinSyncRepoNotContainsAny(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected content to exclude %q", snippet)
		}
	}
}
