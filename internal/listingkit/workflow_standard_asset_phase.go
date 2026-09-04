package listingkit

import (
	"context"
	"reflect"

	listingplatform "task-processor/internal/listing/platform"
	productasset "task-processor/internal/product/asset"
)

type standardWorkflowAssetPhase struct {
	approvedAssets ApprovedAssetInventoryReader
}

func buildStandardWorkflowAssetPhase(s *service) standardWorkflowAssetPhase {
	return standardWorkflowAssetPhase{approvedAssets: resolveWorkflowApprovedAssets(s)}
}

func (p standardWorkflowAssetPhase) run(ctx context.Context, scope productasset.InventoryScope) (productasset.ApprovedAssetInventory, error) {
	if err := productasset.ValidateInventoryScope(scope); err != nil {
		return productasset.ApprovedAssetInventory{}, err
	}
	if p.approvedAssets == nil {
		return productasset.ApprovedAssetInventory{}, productasset.ErrApprovedAssetsNotReady
	}
	inventory, err := p.approvedAssets.GetApprovedInventory(ctx, scope)
	if err != nil {
		return productasset.ApprovedAssetInventory{}, err
	}
	if inventory.Scope != scope {
		return productasset.ApprovedAssetInventory{}, productasset.ErrRepositoryStateInvalid
	}
	for _, approved := range inventory.Assets {
		if approved.Role == productasset.RoleMain {
			return productasset.CloneApprovedAssetInventory(inventory), nil
		}
	}
	return productasset.ApprovedAssetInventory{}, productasset.ErrApprovedAssetsNotReady
}

// runForPlatforms verifies every selected target has an approval inventory.
// The scalar ListingKit result can only safely consume one common inventory,
// so divergent target inventories fail closed instead of letting one target's
// head stand in for another target.
func (p standardWorkflowAssetPhase) runForPlatforms(ctx context.Context, scope productasset.InventoryScope, platforms []string) (productasset.ApprovedAssetInventory, error) {
	if len(platforms) == 0 {
		return p.run(ctx, scope)
	}
	var common productasset.ApprovedAssetInventory
	for index, platform := range platforms {
		targetScope := scope
		targetScope.TargetPlatform = platform
		inventory, err := p.run(ctx, targetScope)
		if err != nil {
			return productasset.ApprovedAssetInventory{}, err
		}
		if index == 0 {
			common = inventory
			continue
		}
		if !reflect.DeepEqual(common.Assets, inventory.Assets) {
			return productasset.ApprovedAssetInventory{}, productasset.ErrRepositoryStateInvalid
		}
	}
	common.Scope = scope
	return common, nil
}

func selectedInventoryPlatforms(task *Task) []string {
	if task == nil || task.Request == nil {
		return nil
	}
	return listingplatform.NormalizeSupportedPlatforms(task.Request.Platforms)
}
