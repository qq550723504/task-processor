package listingkit

import (
	"context"

	"task-processor/internal/asset"
)

type platformAssetDispatchInventoryPersistPhase struct {
	service *service
}

func buildPlatformAssetDispatchInventoryPersistPhase(s *service) *platformAssetDispatchInventoryPersistPhase {
	return &platformAssetDispatchInventoryPersistPhase{service: s}
}

func (p *platformAssetDispatchInventoryPersistPhase) run(
	ctx context.Context,
	inventory *asset.Inventory,
	returnedAssetCount int,
) {
	if p == nil || p.service == nil || p.service.assetRepo == nil || inventory == nil || returnedAssetCount == 0 {
		return
	}
	_ = p.service.assetRepo.SaveInventory(ctx, inventory)
}
