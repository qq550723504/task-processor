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
	if len(final.AssetBundlesByTarget) > 0 {
		for target, bundle := range final.AssetBundlesByTarget {
			if bundle == nil {
				continue
			}
			final.AssetBundlesByTarget[target] = asset.RebuildBundleWithRecords(bundle, dispatchAssetsForTarget(dispatchAssets, target))
			final.AssetInventorySummariesByTarget[target] = asset.InventorySummaryFromBundle(final.AssetBundlesByTarget[target])
		}
		final.applyCompatibilityAssetProjection(final.compatibilityProjectionTarget())
		return
	}
	final.AssetBundle = asset.RebuildBundleWithRecords(final.AssetBundle, dispatchAssets)
	final.AssetInventorySummary = inventory.Summary
}

func dispatchAssetsForTarget(records []asset.AssetRecord, target string) []asset.AssetRecord {
	matched := make([]asset.AssetRecord, 0, len(records))
	for _, record := range records {
		for _, tag := range record.PlatformTags {
			if tag == target {
				matched = append(matched, record)
				break
			}
		}
	}
	return matched
}
