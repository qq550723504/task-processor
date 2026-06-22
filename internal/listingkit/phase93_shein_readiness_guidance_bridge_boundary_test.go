package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinReadinessGuidanceBridgeCallsMarketplaceWorkspaceDirectly(t *testing.T) {
	t.Parallel()

	assertFileAbsent(t, "workspace/shein/readiness_guidance_bridge.go")

	for _, path := range []string{
		"shein_submit_readiness.go",
		"shein_submit_readiness_types.go",
		"shein_submit_readiness_guidance_support.go",
		"shein_build_validation.go",
		"preview_builder_shein_final_review.go",
		"shein_submit_readiness_taxonomy_test.go",
		"shein_workspace_readiness_test.go",
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
				t.Fatalf("%s should call marketplace SHEIN workspace readiness directly", path)
			}
			if strings.Contains(content, `task-processor/internal/listingkit/workspace/shein`) {
				t.Fatalf("%s should not call ListingKit SHEIN workspace readiness bridge", path)
			}
		})
	}

	assertFileAbsent(t, "shein_submit_readiness_checks_support.go")

	taskListSource := readNamedFunctionSource(t, "task_list_item_support.go", "sheinBlockingKeysWithPod")
	assertSourceContainsAll(t, taskListSource, []string{
		"return uniqueNonEmptyStrings(sheinworkspace.FindKeys(readiness.BlockingItems))",
	})
}
