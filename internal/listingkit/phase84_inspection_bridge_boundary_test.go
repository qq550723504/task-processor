package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinInspectionBridgeCallsMarketplaceWorkspaceDirectly(t *testing.T) {
	t.Parallel()

	assertFileAbsent(t, "workspace/shein/inspection_bridge.go")

	assertFileAbsent(t, "shein_workspace_inspection_bridge.go")

	for _, path := range []string{
		"revision_workspace_bridge.go",
		"revision_apply_test.go",
		"shein_review_state.go",
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

	stateSource := readNamedFunctionSource(t, "revision_workspace_bridge.go", "buildRevisionHistoryRestoreStateInput")
	assertSourceContainsAll(t, stateSource, []string{
		"CategoryResolved:      sheinworkspace.IsCategoryResolved(result.Shein)",
		"AttributeResolved:     sheinworkspace.IsAttributeResolved(result.Shein)",
		"SaleAttributeResolved: sheinworkspace.IsSaleAttributeResolved(result.Shein)",
		"ManualReviewNotes:     sheinworkspace.FilterManualReviewNotes(result.Shein.ReviewNotes)",
	})
}
