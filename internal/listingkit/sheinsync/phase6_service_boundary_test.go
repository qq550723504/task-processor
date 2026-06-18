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

func TestSheinSyncActivityAdapterUsesLocalPromotionStrategyContract(t *testing.T) {
	t.Parallel()

	activityFile := readSheinSyncServiceFileContent(t, "activity_adapter.go")
	assertSheinSyncServiceContainsAll(t, activityFile,
		"type SheinPromotionStrategyProvider interface {",
		"GetPromotionStrategy(ctx context.Context, storeID int64, activityKey string) (*SheinPromotionStrategy, error)",
	)
	assertSheinSyncServiceNotContainsAny(t, activityFile,
		`"task-processor/internal/infra/clients/management/api"`,
		"managementapi.OperationStrategyDTO",
	)

	strategyFile := readSheinSyncServiceFileContent(t, "promotion_strategy.go")
	assertSheinSyncServiceContainsAll(t, strategyFile,
		"type SheinPromotionStrategy struct {",
		"type SheinPromotionStrategyInput struct {",
		"func NewSheinPromotionStrategy(",
	)
	assertSheinSyncServiceNotContainsAny(t, strategyFile,
		`"task-processor/internal/infra/clients/management/api"`,
		"managementapi.OperationStrategyDTO",
	)
}

func TestSheinSyncActivityAdapterUsesLocalPromotionBridgeContract(t *testing.T) {
	t.Parallel()

	activityFile := readSheinSyncServiceFileContent(t, "activity_adapter.go")
	assertSheinSyncServiceContainsAll(t, activityFile,
		"type SheinPromotionBridge interface {",
		"type SheinPromotionBridgeFactory interface {",
	)
	assertSheinSyncServiceNotContainsAny(t, activityFile,
		`"task-processor/internal/shein/activity"`,
		"activity.PromotionRegistrationBridge",
		"activity.PromotionRegistrationResult",
	)

	legacyAdapterFile := readSheinSyncServiceFileContent(t, "promotion_bridge_legacy_adapter.go")
	assertSheinSyncServiceContainsAll(t, legacyAdapterFile,
		`"task-processor/internal/shein/activity"`,
		"func NewSheinActivityAdapter(",
		"func NewSheinActivityAdapterWithFactory(",
		"type legacyPromotionBridgeAdapter struct",
	)
}

func TestSheinSyncEnrollmentTestsUseLocalPromotionContracts(t *testing.T) {
	t.Parallel()

	testFile := readSheinSyncServiceFileContent(t, "enrollment_service_test.go")
	assertSheinSyncServiceNotContainsAny(t, testFile,
		`"task-processor/internal/infra/clients/management/api"`,
		`"task-processor/internal/shein/activity"`,
		"managementapi.OperationStrategyDTO",
		"activity.PromotionRegistrationResult",
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
