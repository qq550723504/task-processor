package listingkit

func applyAssetGenerationActionImpactStatesClone(impact *AssetGenerationActionImpact, cloned *AssetGenerationActionImpact) {
	if impact == nil || cloned == nil {
		return
	}
	cloned.States = append([]string(nil), impact.States...)
}
