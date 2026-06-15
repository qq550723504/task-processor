package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStudioAsyncJobHandlerFilesOwnSeparatedFamilies(t *testing.T) {
	t.Parallel()

	dir := "."

	rootFile := readStudioAsyncJobFileContent(t, filepath.Join(dir, "studio_async_jobs_handler.go"))
	storeFile := readStudioAsyncJobFileContent(t, filepath.Join(dir, "studio_async_jobs_handler_store.go"))
	entrypointsFile := readStudioAsyncJobFileContent(t, filepath.Join(dir, "studio_async_jobs_handler_entrypoints.go"))
	runnerFile := readStudioAsyncJobFileContent(t, filepath.Join(dir, "studio_async_jobs_handler_runner.go"))
	supportFile := readStudioAsyncJobFileContent(t, filepath.Join(dir, "studio_async_jobs_handler_support.go"))

	assertStudioAsyncJobContainsAll(t, rootFile,
		"type startStudioAsyncJobRequest struct {",
		"type studioAsyncJob struct {",
		"type studioAsyncJobStoreService interface {",
	)
	assertStudioAsyncJobNotContainsAny(t, rootFile,
		"func newStudioAsyncJobStore(",
		"func (h *handler) StartStudioAsyncJob(",
		"func (h *handler) runStudioAsyncJob(",
	)

	assertStudioAsyncJobContainsAll(t, storeFile,
		"func newStudioAsyncJobStore(",
		"func (s *studioAsyncJobStore) create(",
		"func mapStudioAsyncJobRecord(",
		"func newStudioAsyncJobID() string",
	)
	assertStudioAsyncJobNotContainsAny(t, storeFile,
		"func (h *handler) StartStudioAsyncJob(",
		"func (h *handler) runStudioAsyncJob(",
	)

	assertStudioAsyncJobContainsAll(t, entrypointsFile,
		"func (h *handler) StartStudioAsyncJob(",
		"func (h *handler) GetStudioAsyncJob(",
	)
	assertStudioAsyncJobNotContainsAny(t, entrypointsFile,
		"func newStudioAsyncJobStore(",
		"func (h *handler) runStudioAsyncJob(",
	)

	assertStudioAsyncJobContainsAll(t, runnerFile,
		"var executeStudioDesignBatch = listingkit.ExecuteStudioDesignBatch",
		"func (h *handler) runStudioAsyncJob(",
	)
	assertStudioAsyncJobNotContainsAny(t, runnerFile,
		"func newStudioAsyncJobStore(",
		"func (h *handler) StartStudioAsyncJob(",
	)

	assertStudioAsyncJobContainsAll(t, supportFile,
		"var studioAsyncJobLogger = corelogger.GetGlobalLogger(",
		"func (h *handler) syncStudioDesignAsyncJobSession(",
		"func studioAsyncLogFields(",
		"func requestBaseURL(",
	)
	assertStudioAsyncJobNotContainsAny(t, supportFile,
		"func newStudioAsyncJobStore(",
		"func (h *handler) StartStudioAsyncJob(",
		"func (h *handler) runStudioAsyncJob(",
	)
}

func readStudioAsyncJobFileContent(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func assertStudioAsyncJobContainsAll(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected content to contain %q", snippet)
		}
	}
}

func assertStudioAsyncJobNotContainsAny(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected content to exclude %q", snippet)
		}
	}
}
