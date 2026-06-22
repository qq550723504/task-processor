package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinOverviewBridgeCallsMarketplaceWorkspaceDirectly(t *testing.T) {
	t.Parallel()

	assertFileAbsent(t, "workspace/shein/overview_bridge.go")

	for _, path := range []string{
		"generation_platform_cards.go",
		"model_task.go",
		"platform_payload_input_models.go",
		"platform_payload_models_preview_shein.go",
		"preview_builder_shein_payload.go",
		"submit_readiness_projection_shein.go",
		"task_contract.go",
		"task_list_item_support.go",
		"shein_workspace_overview_test.go",
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
				t.Fatalf("%s should call marketplace SHEIN workspace overview directly", path)
			}
			if strings.Contains(content, `task-processor/internal/listingkit/workspace/shein`) {
				t.Fatalf("%s should not call ListingKit SHEIN workspace overview bridge", path)
			}
		})
	}

	taskListSource := readNamedFunctionSource(t, "task_list_item_support.go", "deriveSheinWorkQueue")
	assertSourceContainsAll(t, taskListSource, []string{
		"return sheinworkspace.BuildTaskWorkQueue(string(task.Status), workflowStatus, overview)",
	})

	previewSource := readNamedFunctionSource(t, "preview_builder_shein.go", "buildSheinPreviewPayloadInput")
	assertSourceContainsAll(t, previewSource, []string{
		"repairState := sheinworkspace.BuildRepairStateInput(repairCenter)",
		"workspaceOverview: sheinworkspace.BuildWorkspaceOverview(statusOverview, submitState, repairState)",
	})

	taxonomySource := readNamedFunctionSource(t, "task_contract.go", "BuildTaskListTaxonomy")
	assertSourceContainsAll(t, taxonomySource, []string{
		"SheinWorkflowStatuses: cloneTaskFacetDescriptorsFromWorkspace(sheinworkspace.WorkflowStatusDescriptors())",
		"SheinWorkQueues:       cloneTaskFacetDescriptorsFromWorkspace(sheinworkspace.WorkQueueDescriptors())",
		"SheinActionQueues:     cloneTaskFacetDescriptorsFromWorkspace(sheinworkspace.ActionQueueDescriptors())",
	})
}
