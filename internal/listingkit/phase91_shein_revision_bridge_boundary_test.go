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
		"revision_history_compare.go",
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

	source := readNamedFunctionSource(t, "revision_history_compare.go", "buildCurrentHistoryCompareDraft")
	assertSourceContainsAll(t, source, []string{
		"return sheinworkspace.BuildEditorRevisionSkeleton(",
		"sheinworkspace.BuildCategoryResolutionPatch(pkg)",
		"sheinworkspace.BuildAttributeResolutionPatch(pkg)",
		"sheinworkspace.BuildSaleAttributeResolutionPatch(pkg)",
		"sheinworkspace.BuildEditorSKCPatches(pkg)",
	})

	src, err := os.ReadFile("shein_workspace_revision_bridge.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_workspace_revision_bridge.go) error = %v", err)
	}
	content := string(src)
	for _, forbidden := range []string{
		"func buildSheinEditorRevisionSkeleton(",
		"func buildSheinMinimalRevisionSkeleton(",
		"func cloneSheinEditorRevisionSkeleton(",
		"func cloneHistorySheinRevisionInput(",
		"func pruneSheinRevisionInput(",
		"func isEmptySheinRevisionInput(",
		"func buildSheinCategoryResolutionPatch(",
		"func buildSheinAttributeResolutionPatch(",
		"func buildSheinSaleAttributeResolutionPatch(",
		"func buildSheinEditorSKCPatches(",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("shein_workspace_revision_bridge.go should not keep unused revision wrapper %q", forbidden)
		}
	}
}
