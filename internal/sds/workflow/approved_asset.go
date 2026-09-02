package workflow

import (
	"context"

	productasset "task-processor/internal/product/asset"
)

// SelectApprovedDesignAsset selects an explicitly assigned approved asset for
// SDS design synchronization. Gallery assets are never implicit design input.
func SelectApprovedDesignAsset(inventory productasset.ApprovedAssetInventory) (productasset.ApprovedAsset, error) {
	for _, role := range []productasset.Role{
		productasset.RoleDesign,
		productasset.RoleMain,
		productasset.RoleWhiteBackground,
	} {
		for _, approved := range inventory.Assets {
			if approved.Role == role {
				return approved, nil
			}
		}
	}
	return productasset.ApprovedAsset{}, productasset.ErrApprovedAssetsNotReady
}

// SyncDesignFromApprovedAssets selects only an explicitly assigned approved
// asset and synchronizes it to SDS.
func (s *Service) SyncDesignFromApprovedAssets(ctx context.Context, input SyncInput, inventory productasset.ApprovedAssetInventory) (*SyncResult, error) {
	approved, err := SelectApprovedDesignAsset(inventory)
	if err != nil {
		return nil, err
	}
	return s.syncDesignFromApprovedAsset(ctx, input, approved)
}
