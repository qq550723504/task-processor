package sheinsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSheinSyncServiceFilesOwnSeparatedFamilies(t *testing.T) {
	t.Parallel()

	dir := "."

	rootFile := readSheinSyncServiceFileContent(t, filepath.Join(dir, "service.go"))
	syncFile := readSheinSyncServiceFileContent(t, filepath.Join(dir, "service_sync.go"))
	snapshotFile := readSheinSyncServiceFileContent(t, filepath.Join(dir, "service_snapshots.go"))
	recordFile := readSheinSyncServiceFileContent(t, filepath.Join(dir, "service_records.go"))

	assertSheinSyncServiceContainsAll(t, rootFile,
		"type SheinSyncService interface {",
		"type sheinSyncService struct {",
		"func NewSheinSyncService(",
		"func NewSheinSyncServiceWithBuilder(",
	)
	assertSheinSyncServiceNotContainsAny(t, rootFile,
		"func (s *sheinSyncService) SyncSheinOnShelfProducts(",
		"func (s *sheinSyncService) fetchSupplementalSnapshots(",
		"func buildSyncedProductRecord(",
	)

	assertSheinSyncServiceContainsAll(t, syncFile,
		"func (s *sheinSyncService) SyncSheinOnShelfProducts(",
		"func (s *sheinSyncService) runSyncJob(",
		"func (s *sheinSyncService) fetchOnShelfProducts(",
		"func (s *sheinSyncService) resolveCostResolver(",
	)
	assertSheinSyncServiceNotContainsAny(t, syncFile,
		"func (s *sheinSyncService) fetchSupplementalSnapshots(",
		"func buildSyncedProductRecord(",
	)

	assertSheinSyncServiceContainsAll(t, snapshotFile,
		"type sheinProductSnapshots struct {",
		"func (s *sheinSyncService) fetchSupplementalSnapshots(",
		"func buildSheinPriceSnapshot(",
		"func buildSheinInventorySnapshot(",
	)
	assertSheinSyncServiceNotContainsAny(t, snapshotFile,
		"func (s *sheinSyncService) SyncSheinOnShelfProducts(",
		"func buildSyncedProductRecord(",
	)

	assertSheinSyncServiceContainsAll(t, recordFile,
		"func buildSyncedProductRecord(",
		"func buildSheinSiteSnapshot(",
		"func parseSheinSyncTime(",
		"func countDeactivatedProducts(",
	)
	assertSheinSyncServiceNotContainsAny(t, recordFile,
		"func (s *sheinSyncService) SyncSheinOnShelfProducts(",
		"func (s *sheinSyncService) fetchSupplementalSnapshots(",
	)
}

func readSheinSyncServiceFileContent(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func assertSheinSyncServiceContainsAll(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected content to contain %q", snippet)
		}
	}
}

func assertSheinSyncServiceNotContainsAny(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected content to exclude %q", snippet)
		}
	}
}
