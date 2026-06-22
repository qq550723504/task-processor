package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinRevisionHistoryBridgeCallsMarketplaceWorkspaceDirectly(t *testing.T) {
	t.Parallel()

	assertFileAbsent(t, "workspace/shein/revision_history_bridge.go")

	for _, path := range []string{
		"revision_history_compare.go",
		"revision_history_detail.go",
		"revision_workspace_bridge.go",
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

	source := readNamedFunctionSource(t, "revision_workspace_bridge.go", "buildRevisionRestorePreviewFromDetail")
	assertSourceContainsAll(t, source, []string{
		"sheinworkspace.BuildRevisionDiffPreviewFromInput(detail.RestorePayload.Core.Draft)",
		"return sheinworkspace.RebuildRestorePreviewPayload(detail.RestorePayload, compare)",
	})

	detailSource := readNamedFunctionSource(t, "revision_history_detail.go", "buildRevisionHistoryDetail")
	assertSourceContainsAll(t, detailSource, []string{
		"sheinworkspace.BuildHistoryNavigation(",
	})

	workspaceBridgeSrc, err := os.ReadFile("revision_workspace_bridge.go")
	if err != nil {
		t.Fatalf("ReadFile(revision_workspace_bridge.go) error = %v", err)
	}
	workspaceBridgeContent := string(workspaceBridgeSrc)
	for _, forbidden := range []string{
		"func buildRevisionHistoryNavigation(",
		"func buildRevisionHistoryRestoreMessages(",
		"return sheinworkspace.BuildHistoryNavigation(prevRevisionID, nextRevisionID)",
		"return sheinworkspace.BuildHistoryRestoreMessages(context, safety, overview)",
	} {
		if strings.Contains(workspaceBridgeContent, forbidden) {
			t.Fatalf("revision_workspace_bridge.go should not keep revision history wrapper %q", forbidden)
		}
	}
}
