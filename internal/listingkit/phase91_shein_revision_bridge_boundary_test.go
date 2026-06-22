package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinRevisionBridgeCallsMarketplaceWorkspaceDirectly(t *testing.T) {
	t.Parallel()

	assertFileAbsent(t, "workspace/shein/revision_bridge.go")

	for _, path := range []string{
		"shein_workspace_revision_bridge.go",
		"revision_history_restore_draft.go",
		"revision_restore_request.go",
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

	source := readNamedFunctionSource(t, "shein_workspace_revision_bridge.go", "buildSheinEditorRevisionSkeleton")
	assertSourceContainsAll(t, source, []string{
		"return sheinworkspace.BuildEditorRevisionSkeleton(",
		"buildSheinCategoryResolutionPatch(pkg)",
		"buildSheinAttributeResolutionPatch(pkg)",
		"buildSheinSaleAttributeResolutionPatch(pkg)",
		"buildSheinEditorSKCPatches(pkg)",
	})
}
