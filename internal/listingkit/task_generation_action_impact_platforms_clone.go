package listingkit

func applyAssetGenerationActionImpactPlatformsClone(impact *AssetGenerationActionImpact, cloned *AssetGenerationActionImpact) {
	if impact == nil || cloned == nil {
		return
	}
	cloned.Platforms = append([]string(nil), impact.Platforms...)
}
