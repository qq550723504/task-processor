package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSheinSyncSummaryHandlerFilesOwnSeparatedFamilies(t *testing.T) {
	t.Parallel()

	dir := "."

	rootFile := readSheinSyncSummaryFileContent(t, filepath.Join(dir, "shein_sync_summary_handler.go"))
	dashboardFile := readSheinSyncSummaryFileContent(t, filepath.Join(dir, "shein_sync_summary_handler_dashboard.go"))
	runsFile := readSheinSyncSummaryFileContent(t, filepath.Join(dir, "shein_sync_summary_handler_runs.go"))
	supportFile := readSheinSyncSummaryFileContent(t, filepath.Join(dir, "shein_sync_summary_handler_support.go"))

	assertSheinSyncSummaryContainsAll(t, rootFile,
		"const (",
		"type sheinSummaryQuery struct {",
		"type listSheinEnrollmentRunsQuery struct {",
	)
	assertSheinSyncSummaryNotContainsAny(t, rootFile,
		"func (h *handler) ListSheinEnrollmentDashboard(",
		"func (h *handler) ListSheinActivityEnrollmentRuns(",
		"func resolveSheinSummaryActivityType(",
	)

	assertSheinSyncSummaryContainsAll(t, dashboardFile,
		"func (h *handler) ListSheinEnrollmentDashboard(",
		"func (h *handler) GetSheinEnrollmentStoreSummary(",
	)
	assertSheinSyncSummaryNotContainsAny(t, dashboardFile,
		"func (h *handler) ListSheinActivityEnrollmentRuns(",
		"func resolveSheinSummaryActivityType(",
	)

	assertSheinSyncSummaryContainsAll(t, runsFile,
		"func (h *handler) ListSheinActivityEnrollmentRuns(",
	)
	assertSheinSyncSummaryNotContainsAny(t, runsFile,
		"func (h *handler) ListSheinEnrollmentDashboard(",
		"func resolveSheinSummaryActivityType(",
	)

	assertSheinSyncSummaryContainsAll(t, supportFile,
		"func resolveSheinSummaryActivityType(",
		"func (h *handler) listSheinStores(",
		"func (h *handler) buildSheinEnrollmentStoreSummary(",
		"func summarizeLatestSheinCandidates(",
	)
	assertSheinSyncSummaryNotContainsAny(t, supportFile,
		"func (h *handler) ListSheinEnrollmentDashboard(",
		"func (h *handler) ListSheinActivityEnrollmentRuns(",
	)
}

func readSheinSyncSummaryFileContent(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func assertSheinSyncSummaryContainsAll(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected content to contain %q", snippet)
		}
	}
}

func assertSheinSyncSummaryNotContainsAny(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected content to exclude %q", snippet)
		}
	}
}
