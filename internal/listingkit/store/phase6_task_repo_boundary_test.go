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
	assertTaskRepoNotContainsAny(t, rootFile,
		"func (r *taskRepository) ListTasks(",
		"func (r *taskRepository) MarkProcessing(",
		"func (r *taskRepository) GetCanonicalProductCache(",
		"func applyTaskAccessScope(",
	)

	assertTaskRepoContainsAll(t, listingFile,
		"func (r *taskRepository) CreateTask(",
		"func (r *taskRepository) ListTasks(",
		"func (r *taskRepository) ListTaskSummaryTasks(",
		"func matchesTaskListFilterRow(",
	)
	assertTaskRepoNotContainsAny(t, listingFile,
		"func (r *taskRepository) MarkProcessing(",
		"func (r *taskRepository) GetCanonicalProductCache(",
		"func applyTaskAccessScope(",
	)

	assertTaskRepoContainsAll(t, statusFile,
		"func (r *taskRepository) MarkProcessing(",
		"func (r *taskRepository) RecoverBlockedTaskNow(",
		"func (r *taskRepository) MutateTaskResult(",
		"func collectRecoverableTasks(",
	)
	assertTaskRepoNotContainsAny(t, statusFile,
		"func (r *taskRepository) ListTasks(",
		"func (r *taskRepository) GetCanonicalProductCache(",
		"func applyTaskAccessScope(",
	)

	assertTaskRepoContainsAll(t, cacheFile,
		"func (r *taskRepository) GetCanonicalProductCache(",
		"func (r *taskRepository) SaveCanonicalProductCache(",
		"func (r *taskRepository) GetSDSBaselineCache(",
		"func storedCanonicalFingerprint(",
	)
	assertTaskRepoNotContainsAny(t, cacheFile,
		"func (r *taskRepository) ListTasks(",
		"func (r *taskRepository) MarkProcessing(",
		"func applyTaskAccessScope(",
	)

	assertTaskRepoContainsAll(t, scopeFile,
		"func tenantScope(",
		"func applyTaskAccessScope(",
		"func taskVisibleToUser(",
		"func currentTimestampValue(",
	)
	assertTaskRepoNotContainsAny(t, scopeFile,
		"func (r *taskRepository) ListTasks(",
		"func (r *taskRepository) MarkProcessing(",
		"func (r *taskRepository) GetCanonicalProductCache(",
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
