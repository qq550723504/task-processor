package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinSubmitBridgeCallsMarketplaceWorkspaceDirectly(t *testing.T) {
	t.Parallel()

	assertFileAbsent(t, "workspace/shein/submit_bridge.go")

	src, err := os.ReadFile("shein_workspace_submit_bridge.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_workspace_submit_bridge.go) error = %v", err)
	}
	content := string(src)
	if !strings.Contains(content, `sheinworkspace "task-processor/internal/marketplace/shein/workspace"`) {
		t.Fatal("shein_workspace_submit_bridge.go should call marketplace SHEIN workspace directly")
	}
	if strings.Contains(content, `task-processor/internal/listingkit/workspace/shein`) {
		t.Fatal("shein_workspace_submit_bridge.go should not call ListingKit SHEIN workspace bridge")
	}

	if strings.Contains(content, "func buildSheinSubmitChecklist(") {
		t.Fatal("shein_workspace_submit_bridge.go should not keep submit checklist wrapper")
	}
}
