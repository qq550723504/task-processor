package listingkit

func applyAssetGenerationActionImpactCloneShape(impact *AssetGenerationActionImpact, cloned *AssetGenerationActionImpact) {
	if impact == nil || cloned == nil {
		return
	}
	applyAssetGenerationActionImpactSliceClone(impact, cloned)
}
