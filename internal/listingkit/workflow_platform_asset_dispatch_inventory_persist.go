package listingkit

import (
	"context"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
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
	dispatchResult *assetgeneration.Result,
) {
	if p == nil || p.service == nil || p.service.assetRepo == nil || inventory == nil || dispatchResult == nil || len(dispatchResult.Assets) == 0 {
		return
	}
	_ = p.service.assetRepo.SaveInventory(ctx, inventory)
}
