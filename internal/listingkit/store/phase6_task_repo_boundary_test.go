package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskRepositoryFilesOwnSeparatedFamilies(t *testing.T) {
	t.Parallel()

	dir := "."
	rootFile := readTaskRepoFileContent(t, filepath.Join(dir, "task_repo.go"))
	listingFile := readTaskRepoFileContent(t, filepath.Join(dir, "task_repo_listing.go"))
	statusFile := readTaskRepoFileContent(t, filepath.Join(dir, "task_repo_status.go"))
	cacheFile := readTaskRepoFileContent(t, filepath.Join(dir, "task_repo_cache.go"))
	scopeFile := readTaskRepoFileContent(t, filepath.Join(dir, "task_repo_scope.go"))

	assertTaskRepoContainsAll(t, rootFile,
		"type taskRepository struct {",
		"func NewTaskRepository(db *gorm.DB) listingkit.Repository {",
	)
	assertTaskRepoContainsAll(t, listingFile,
		"func (r *taskRepository) CreateTask(",
		"func (r *taskRepository) ListTasks(",
		"func (r *taskRepository) ListTaskSummaryTasks(",
		"func matchesTaskListFilterRow(",
	)
	assertTaskRepoContainsAll(t, statusFile,
		"func (r *taskRepository) MarkProcessing(",
		"func (r *taskRepository) RecoverBlockedTaskNow(",
		"func (r *taskRepository) MutateTaskResult(",
		"func collectRecoverableTasks(",
	)
	assertTaskRepoContainsAll(t, cacheFile,
		"func (r *taskRepository) GetSDSBaselineCache(",
		"func (r *taskRepository) SaveSDSBaselineCache(",
	)
	assertTaskRepoContainsAll(t, scopeFile,
		"func tenantScope(",
		"func applyTaskAccessScope(",
		"func taskVisibleToUser(",
		"func currentTimestampValue(",
	)

	for _, content := range []string{rootFile, listingFile, statusFile, cacheFile, scopeFile} {
		assertTaskRepoNotContainsAny(t, content,
			"GetCanonicalProductCache(",
			"SaveCanonicalProductCache(",
			"storedCanonicalFingerprint(",
		)
	}
}

func TestTaskRepositoryRecoveryDelegatesRetryableBlockPolicy(t *testing.T) {
	t.Parallel()

	statusFile := readTaskRepoFileContent(t, "task_repo_status.go")
	memStoreFile := readTaskRepoFileContent(t, "mem_store.go")

	assertTaskRepoContainsAll(t, statusFile, "listingkit.BuildRecoveredRetryableBlock(")
	assertTaskRepoNotContainsAny(t, statusFile,
		"block.LastRetryAt = timestampPointer(effectiveRecoveredAt)",
		"block.NextRetryAt = nil",
		"block.AutoRetryPaused = false",
	)
	assertTaskRepoContainsAll(t, memStoreFile, "listingkit.BuildRecoveredRetryableBlock(")
	assertTaskRepoNotContainsAny(t, memStoreFile,
		"block.LastRetryAt = timestampPointer(effectiveRecoveredAt)",
		"block.NextRetryAt = nil",
		"block.AutoRetryPaused = false",
	)
}

func readTaskRepoFileContent(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func assertTaskRepoContainsAll(t *testing.T, content string, snippets ...string) {
	t.Helper()
	for _, snippet := range snippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected content to contain %q", snippet)
		}
	}
}

func assertTaskRepoNotContainsAny(t *testing.T, content string, snippets ...string) {
	t.Helper()
	for _, snippet := range snippets {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected content to exclude %q", snippet)
		}
	}
}
