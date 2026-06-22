package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinEditorBridgeCallsMarketplaceWorkspaceDirectly(t *testing.T) {
	t.Parallel()

	assertFileAbsent(t, "workspace/shein/editor_bridge.go")

	src, err := os.ReadFile("shein_workspace_editor_bridge.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_workspace_editor_bridge.go) error = %v", err)
	}
	content := string(src)
	if !strings.Contains(content, `sheinworkspace "task-processor/internal/marketplace/shein/workspace"`) {
		t.Fatal("shein_workspace_editor_bridge.go should call marketplace SHEIN workspace directly")
	}
	if strings.Contains(content, `task-processor/internal/listingkit/workspace/shein`) {
		t.Fatal("shein_workspace_editor_bridge.go should not call ListingKit SHEIN workspace bridge")
	}

	progressSource := readNamedFunctionSource(t, "shein_workspace_editor_bridge.go", "buildSheinEditorProgress")
	assertSourceContainsAll(t, progressSource, []string{
		"return sheinworkspace.BuildEditorProgress(pkg, sheinworkspace.ChecklistItemCount(checklist))",
	})
}
