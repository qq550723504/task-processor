package listingkit

import (
	"context"

	"task-processor/internal/asset"
	assetrepo "task-processor/internal/asset/repository"
)

type platformAssetDispatchInventoryPersistPhase struct {
	assetRepository assetrepo.Repository
}

func buildPlatformAssetDispatchInventoryPersistPhase(s *service) *platformAssetDispatchInventoryPersistPhase {
	return &platformAssetDispatchInventoryPersistPhase{assetRepository: resolveWorkflowAssetRepository(s)}
}

func (p *platformAssetDispatchInventoryPersistPhase) run(
	ctx context.Context,
	inventory *asset.Inventory,
	returnedAssetCount int,
) {
	if p == nil || p.assetRepository == nil || inventory == nil || returnedAssetCount == 0 {
		return
	}
	_ = p.assetRepository.SaveInventory(ctx, inventory)
}
