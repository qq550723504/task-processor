package listingkit

import (
	"context"

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

// runForPlatforms verifies every selected target has an approval inventory and
// retains each inventory under its canonical target platform key.
func (p standardWorkflowAssetPhase) runForPlatforms(ctx context.Context, scope productasset.InventoryScope, platforms []string) (map[string]productasset.ApprovedAssetInventory, error) {
	if len(platforms) == 0 {
		inventory, err := p.run(ctx, scope)
		if err != nil {
			return nil, err
		}
		return map[string]productasset.ApprovedAssetInventory{"": inventory}, nil
	}
	inventories := make(map[string]productasset.ApprovedAssetInventory, len(platforms))
	for _, platform := range platforms {
		targetScope := scope
		targetScope.TargetPlatform = platform
		inventory, err := p.run(ctx, targetScope)
		if err != nil {
			return nil, err
		}
		inventories[platform] = inventory
	}
	return inventories, nil
}

func selectedInventoryPlatforms(task *Task) []string {
	if task == nil || task.Request == nil {
		return nil
	}
	return listingplatform.NormalizeSupportedPlatforms(task.Request.Platforms)
}
