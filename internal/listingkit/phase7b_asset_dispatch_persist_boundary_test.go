package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestWorkflowPlatformAssetDispatchInventoryPersistFileOwnsInventoryDurability(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("workflow_platform_asset_dispatch_inventory_persist.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_platform_asset_dispatch_inventory_persist.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"func (p *platformAssetDispatchInventoryPersistPhase) run(",
		"returnedAssetCount int",
		"_ = p.service.mirrors.assetRepo.SaveInventory(ctx, inventory)",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_inventory_persist.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"decorateListingKitResultGeneration(",
		"SaveGenerationTasks(",
		"mergeGenerationTasks(",
		"rebuildInventorySummary(",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_inventory_persist.go should not contain %q", needle)
		}
	}
}

func TestWorkflowPlatformAssetDispatchInventoryPersistFileOnlyDurableWhenReturnedAssetsPositive(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("workflow_platform_asset_dispatch_inventory_persist.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_platform_asset_dispatch_inventory_persist.go) error = %v", err)
	}
	content := string(src)

	if !strings.Contains(content, "returnedAssetCount == 0") {
		t.Fatal("workflow_platform_asset_dispatch_inventory_persist.go should gate inventory durability on returned assets > 0")
	}
	for _, needle := range []string{
		"len(inventory.Records)",
		"GeneratedRecords",
		"inventory.Summary",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_inventory_persist.go should not derive durability from %q", needle)
		}
	}
}
