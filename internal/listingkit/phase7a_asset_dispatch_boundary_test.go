package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestWorkflowPlatformAssetDispatchPhaseFileDelegatesToOrchestrationHelpers(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("workflow_platform_asset_dispatch_phase.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_platform_asset_dispatch_phase.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"p.preAttachBundles(",
		"collectPlatformGenerationTasks(final)",
		"p.dispatchAndApply(",
		"p.persistInventory(",
		"p.persistHandoff(",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_phase.go should contain %q", needle)
		}
	}

	preAttachIndex := strings.Index(content, "p.preAttachBundles(")
	collectIndex := strings.Index(content, "collectPlatformGenerationTasks(final)")
	dispatchIndex := strings.Index(content, "p.dispatchAndApply(")
	inventoryPersistIndex := strings.Index(content, "p.persistInventory(")
	persistIndex := strings.Index(content, "p.persistHandoff(")
	if !(preAttachIndex < collectIndex && collectIndex < dispatchIndex && dispatchIndex < inventoryPersistIndex && inventoryPersistIndex < persistIndex) {
		t.Fatalf("workflow_platform_asset_dispatch_phase.go should keep pre-attach -> collect -> dispatch/apply -> inventory persist -> persist handoff order")
	}

	for _, needle := range []string{
		"inventory.Records = append(",
		"rebuildInventorySummary(",
		"mergeGenerationTasks(",
		"decorateListingKitResultGeneration(",
		"SaveGenerationTasks(",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_phase.go should not contain %q", needle)
		}
	}

	if strings.Contains(content, "_ = p.service.mirrors.assetRepo.SaveInventory(ctx, inventory)") {
		t.Fatal("workflow_platform_asset_dispatch_phase.go should hand off inventory durability instead of calling SaveInventory inline")
	}
}

func TestWorkflowPlatformAssetDispatchMutationAndPersistFilesOwnTheirSideEffects(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		file        string
		shouldOwn   []string
		shouldAvoid []string
	}{
		{
			file: "workflow_platform_asset_dispatch_apply.go",
			shouldOwn: []string{
				"buildPlatformAssetDispatchInventoryApplyPhase().run(",
				"buildPlatformAssetDispatchBundleApplyPhase(bundleBuilder).run(",
			},
			shouldAvoid: []string{
				"inventory.Records = append(",
				"rebuildInventorySummary(",
				"rebuildBundleWithGeneratedAssets(",
				"attachPlatformImageBundles(",
				"mergeGenerationTasks(",
				"decorateListingKitResultGeneration(",
				"SaveGenerationTasks(",
			},
		},
		{
			file: "workflow_platform_asset_dispatch_persist.go",
			shouldOwn: []string{
				"decorateListingKitResultGeneration(",
				"SaveGenerationTasks(",
			},
			shouldAvoid: []string{
				"inventory.Records = append(",
				"rebuildInventorySummary(",
				"mergeGenerationTasks(",
			},
		},
	} {
		src, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", tc.file, err)
		}
		content := string(src)

		for _, needle := range tc.shouldOwn {
			if !strings.Contains(content, needle) {
				t.Fatalf("%s should contain %q", tc.file, needle)
			}
		}
		for _, needle := range tc.shouldAvoid {
			if strings.Contains(content, needle) {
				t.Fatalf("%s should not contain %q", tc.file, needle)
			}
		}
	}
}
