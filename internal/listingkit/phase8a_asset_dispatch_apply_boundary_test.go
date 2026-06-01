package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestWorkflowPlatformAssetDispatchApplyFileDelegatesToMutationSubSeams(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("workflow_platform_asset_dispatch_apply.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_platform_asset_dispatch_apply.go) error = %v", err)
	}
	source := string(content)

	required := []string{
		"buildPlatformAssetDispatchInventoryApplyPhase().run(",
		"buildPlatformAssetDispatchBundleApplyPhase(bundleBuilder).run(",
	}
	for _, needle := range required {
		if !strings.Contains(source, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_apply.go should contain %q", needle)
		}
	}

	forbidden := []string{
		"inventory.Records = append(inventory.Records, dispatchResult.Assets...)",
		"rebuildInventorySummary(inventory)",
		"rebuildBundleWithGeneratedAssets(final.AssetBundle, dispatchResult.Assets)",
		"attachPlatformImageBundles(final, inventory, recipesByPlatform, &assetgeneration.Result{Tasks: dispatchResult.Tasks}, bundleBuilder)",
		"mergeGenerationTasks(generationTasks, dispatchResult.Tasks)",
		"_ = inventory.Records",
		"_ = final.AssetBundle",
	}
	for _, needle := range forbidden {
		if strings.Contains(source, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_apply.go should not inline or retain transition code %q", needle)
		}
	}
}

func TestWorkflowPlatformAssetDispatchMutationSubSeamFilesOwnShapingResponsibilities(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		file        string
		shouldOwn   []string
		shouldAvoid []string
	}{
		{
			file: "workflow_platform_asset_dispatch_inventory_apply.go",
			shouldOwn: []string{
				"inventory.Records = append(",
				"rebuildInventorySummary(",
				"rebuildBundleWithGeneratedAssets(",
			},
			shouldAvoid: []string{
				"attachPlatformImageBundles(",
				"mergeGenerationTasks(",
				"SaveInventory(",
				"SaveGenerationTasks(",
			},
		},
		{
			file: "workflow_platform_asset_dispatch_bundle_apply.go",
			shouldOwn: []string{
				"buildPlatformAssetDispatchBundleReshapePhase(p.bundleBuilder).run(",
				"buildPlatformAssetDispatchTaskMergePhase().run(",
			},
			shouldAvoid: []string{
				"attachPlatformImageBundles(",
				"mergeGenerationTasks(",
				"inventory.Records = append(",
				"rebuildInventorySummary(",
				"rebuildBundleWithGeneratedAssets(",
				"SaveInventory(",
				"SaveGenerationTasks(",
			},
		},
	} {
		content, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", tc.file, err)
		}
		source := string(content)

		for _, needle := range tc.shouldOwn {
			if !strings.Contains(source, needle) {
				t.Fatalf("%s should contain %q", tc.file, needle)
			}
		}
		for _, needle := range tc.shouldAvoid {
			if strings.Contains(source, needle) {
				t.Fatalf("%s should not contain %q", tc.file, needle)
			}
		}
	}
}
