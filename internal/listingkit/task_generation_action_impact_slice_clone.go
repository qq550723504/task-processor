package listingkit

func applyAssetGenerationActionImpactSliceClone(impact *AssetGenerationActionImpact, cloned *AssetGenerationActionImpact) {
	if impact == nil || cloned == nil {
		return
	}
	applyAssetGenerationActionImpactPlatformsClone(impact, cloned)
	applyAssetGenerationActionImpactQualityGradesClone(impact, cloned)
	applyAssetGenerationActionImpactStatesClone(impact, cloned)
}
