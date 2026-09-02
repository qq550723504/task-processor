package listingkit

import (
	"context"

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
