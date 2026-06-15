package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSheinSyncHandlerFilesOwnSeparatedFamilies(t *testing.T) {
	t.Parallel()

	dir := "."

	rootFile := readSheinSyncHandlerFileContent(t, filepath.Join(dir, "shein_sync_handler.go"))
	productsFile := readSheinSyncHandlerFileContent(t, filepath.Join(dir, "shein_sync_handler_products.go"))
	candidatesFile := readSheinSyncHandlerFileContent(t, filepath.Join(dir, "shein_sync_handler_candidates.go"))
	enrollmentFile := readSheinSyncHandlerFileContent(t, filepath.Join(dir, "shein_sync_handler_enrollment.go"))
	supportFile := readSheinSyncHandlerFileContent(t, filepath.Join(dir, "shein_sync_handler_support.go"))
	summaryFile := readSheinSyncHandlerFileContent(t, filepath.Join(dir, "shein_sync_summary_handler.go"))

	assertSheinSyncHandlerContainsAll(t, rootFile,
		"type triggerSheinStoreSyncRequest struct {",
		"type reviewSheinActivityCandidateRequest struct {",
		"type executeSheinActivityEnrollmentRequest struct {",
	)
	assertSheinSyncHandlerNotContainsAny(t, rootFile,
		"func (h *handler) TriggerSheinStoreSync(",
		"func (h *handler) RefreshSheinActivityCandidates(",
		"func parseSheinScopedRequest(",
	)

	assertSheinSyncHandlerContainsAll(t, productsFile,
		"func (h *handler) TriggerSheinStoreSync(",
		"func (h *handler) ListSheinSyncedProducts(",
		"func (h *handler) UpdateSheinSyncedProductCost(",
	)
	assertSheinSyncHandlerNotContainsAny(t, productsFile,
		"func (h *handler) RefreshSheinActivityCandidates(",
		"func (h *handler) ExecuteSheinActivityEnrollment(",
		"func parseSheinScopedRequest(",
	)

	assertSheinSyncHandlerContainsAll(t, candidatesFile,
		"func (h *handler) RefreshSheinActivityCandidates(",
		"func (h *handler) ListSheinActivityCandidates(",
		"func (h *handler) ReviewSheinActivityCandidate(",
	)
	assertSheinSyncHandlerNotContainsAny(t, candidatesFile,
		"func (h *handler) TriggerSheinStoreSync(",
		"func (h *handler) ExecuteSheinActivityEnrollment(",
		"func parseSheinScopedRequest(",
	)

	assertSheinSyncHandlerContainsAll(t, enrollmentFile,
		"func (h *handler) ExecuteSheinActivityEnrollment(",
	)
	assertSheinSyncHandlerNotContainsAny(t, enrollmentFile,
		"func (h *handler) TriggerSheinStoreSync(",
		"func (h *handler) RefreshSheinActivityCandidates(",
		"func parseSheinScopedRequest(",
	)

	assertSheinSyncHandlerContainsAll(t, supportFile,
		"func parseSheinScopedRequest(",
		"func parseSheinTenantID(",
		"func parseSheinInt64Param(",
		"func parseOptionalBoolQuery(",
	)
	assertSheinSyncHandlerNotContainsAny(t, supportFile,
		"func (h *handler) TriggerSheinStoreSync(",
		"func (h *handler) RefreshSheinActivityCandidates(",
		"func (h *handler) ExecuteSheinActivityEnrollment(",
	)

	assertSheinSyncHandlerContainsAll(t, summaryFile,
		"func (h *handler) ListSheinEnrollmentDashboard(",
		"func (h *handler) ListSheinActivityEnrollmentRuns(",
	)
	assertSheinSyncHandlerNotContainsAny(t, summaryFile,
		"func (h *handler) TriggerSheinStoreSync(",
		"func (h *handler) ReviewSheinActivityCandidate(",
		"func parseSheinScopedRequest(",
	)
}

func readSheinSyncHandlerFileContent(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func assertSheinSyncHandlerContainsAll(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected content to contain %q", snippet)
		}
	}
}

func assertSheinSyncHandlerNotContainsAny(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected content to exclude %q", snippet)
		}
	}
}
