package listingkit

import "task-processor/internal/asset"

type platformAssetDispatchInventoryApplyPhase struct{}

func buildPlatformAssetDispatchInventoryApplyPhase() *platformAssetDispatchInventoryApplyPhase {
	return &platformAssetDispatchInventoryApplyPhase{}
}

func (p *platformAssetDispatchInventoryApplyPhase) run(
	final *ListingKitResult,
	inventory *asset.Inventory,
	dispatchAssets []asset.AssetRecord,
) {
	if p == nil || len(dispatchAssets) == 0 {
		return
	}

	inventory.Records = append(inventory.Records, dispatchAssets...)
	inventory.Summary = asset.RebuildInventorySummary(inventory)
	final.AssetBundle = asset.RebuildBundleWithRecords(final.AssetBundle, dispatchAssets)
	final.AssetInventorySummary = inventory.Summary
}
