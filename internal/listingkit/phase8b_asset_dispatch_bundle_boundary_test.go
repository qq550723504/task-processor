package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestWorkflowPlatformAssetDispatchBundleReshapeFileOwnsBundleReshaping(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("workflow_platform_asset_dispatch_bundle_reshape.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_platform_asset_dispatch_bundle_reshape.go) error = %v", err)
	}
	source := string(content)

	for _, needle := range []string{
		"attachPlatformImageBundles(",
	} {
		if !strings.Contains(source, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_bundle_reshape.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"mergeGenerationTasks(",
		"inventory.Records = append(",
		"asset.RebuildInventorySummary(",
		"SaveInventory(",
		"SaveGenerationTasks(",
	} {
		if strings.Contains(source, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_bundle_reshape.go should not contain %q", needle)
		}
	}
}

func TestWorkflowPlatformAssetDispatchTaskMergeFileOwnsReturnedTaskMerge(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("workflow_platform_asset_dispatch_task_merge.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_platform_asset_dispatch_task_merge.go) error = %v", err)
	}
	source := string(content)

	for _, needle := range []string{
		"mergeGenerationTasks(",
	} {
		if !strings.Contains(source, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_task_merge.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"attachPlatformImageBundles(",
		"inventory.Records = append(",
		"asset.RebuildInventorySummary(",
		"SaveInventory(",
		"SaveGenerationTasks(",
	} {
		if strings.Contains(source, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_task_merge.go should not contain %q", needle)
		}
	}
}
