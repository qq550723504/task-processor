package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinRepairBridgeCallsMarketplaceWorkspaceDirectly(t *testing.T) {
	t.Parallel()

	assertFileAbsent(t, "workspace/shein/repair_bridge.go")

	for _, path := range []string{
		"shein_workspace_repair_bridge.go",
		"shein_repair_support.go",
		"shein_repair_center.go",
		"shein_repair_revision_support.go",
	} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%s) error = %v", path, err)
			}
			content := string(src)
			if !strings.Contains(content, `sheinworkspace "task-processor/internal/marketplace/shein/workspace"`) {
				t.Fatalf("%s should call marketplace SHEIN workspace directly", path)
			}
			if strings.Contains(content, `task-processor/internal/listingkit/workspace/shein`) {
				t.Fatalf("%s should not call ListingKit SHEIN workspace bridge", path)
			}
		})
	}
}
