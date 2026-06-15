package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSheinSyncMemRepositoryFilesOwnSeparatedFamilies(t *testing.T) {
	t.Parallel()

	dir := "."

	rootFile := readSheinSyncMemStoreFileContent(t, filepath.Join(dir, "shein_sync_mem_store.go"))
	productFile := readSheinSyncMemStoreFileContent(t, filepath.Join(dir, "shein_sync_mem_store_products.go"))
	jobFile := readSheinSyncMemStoreFileContent(t, filepath.Join(dir, "shein_sync_mem_store_jobs.go"))
	enrollmentFile := readSheinSyncMemStoreFileContent(t, filepath.Join(dir, "shein_sync_mem_store_enrollment.go"))
	filterFile := readSheinSyncMemStoreFileContent(t, filepath.Join(dir, "shein_sync_mem_store_filters.go"))

	assertSheinSyncMemStoreContainsAll(t, rootFile,
		"type MemSheinSyncRepository struct {",
		"func NewMemSheinSyncRepository() listingkit.SheinSyncRepository {",
	)
	assertSheinSyncMemStoreNotContainsAny(t, rootFile,
		"func (r *MemSheinSyncRepository) UpsertSyncedProducts(",
		"func (r *MemSheinSyncRepository) SaveSyncJob(",
		"func (r *MemSheinSyncRepository) SaveCandidates(",
		"func matchesSheinSyncedProductQuery(",
	)

	assertSheinSyncMemStoreContainsAll(t, productFile,
		"func (r *MemSheinSyncRepository) UpsertSyncedProducts(",
		"func (r *MemSheinSyncRepository) ListSyncedProducts(",
		"func (r *MemSheinSyncRepository) UpdateManualCostPrice(",
		"func (r *MemSheinSyncRepository) MarkMissingSyncedProductsInactive(",
	)
	assertSheinSyncMemStoreNotContainsAny(t, productFile,
		"func (r *MemSheinSyncRepository) SaveSyncJob(",
		"func (r *MemSheinSyncRepository) SaveCandidates(",
		"func matchesSheinSyncedProductQuery(",
	)

	assertSheinSyncMemStoreContainsAll(t, jobFile,
		"func (r *MemSheinSyncRepository) SaveSyncJob(",
		"func (r *MemSheinSyncRepository) ListSyncJobs(",
	)
	assertSheinSyncMemStoreNotContainsAny(t, jobFile,
		"func (r *MemSheinSyncRepository) UpsertSyncedProducts(",
		"func (r *MemSheinSyncRepository) SaveCandidates(",
		"func matchesSheinSyncJobQuery(",
	)

	assertSheinSyncMemStoreContainsAll(t, enrollmentFile,
		"func (r *MemSheinSyncRepository) SaveCandidates(",
		"func (r *MemSheinSyncRepository) ListCandidates(",
		"func (r *MemSheinSyncRepository) CreateEnrollmentRun(",
		"func (r *MemSheinSyncRepository) SaveEnrollmentItems(",
	)
	assertSheinSyncMemStoreNotContainsAny(t, enrollmentFile,
		"func (r *MemSheinSyncRepository) UpsertSyncedProducts(",
		"func (r *MemSheinSyncRepository) SaveSyncJob(",
		"func matchesSheinActivityCandidateQuery(",
	)

	assertSheinSyncMemStoreContainsAll(t, filterFile,
		"func matchesSheinSyncedProductQuery(",
		"func matchesSheinSyncJobQuery(",
		"func matchesSheinActivityCandidateQuery(",
		"func cloneSheinEnrollmentRunRecord(",
	)
	assertSheinSyncMemStoreNotContainsAny(t, filterFile,
		"func (r *MemSheinSyncRepository) UpsertSyncedProducts(",
		"func (r *MemSheinSyncRepository) SaveSyncJob(",
		"func (r *MemSheinSyncRepository) SaveCandidates(",
	)
}

func readSheinSyncMemStoreFileContent(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func assertSheinSyncMemStoreContainsAll(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected content to contain %q", snippet)
		}
	}
}

func assertSheinSyncMemStoreNotContainsAny(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected content to exclude %q", snippet)
		}
	}
}
