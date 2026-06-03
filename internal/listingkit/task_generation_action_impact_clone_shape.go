package listingkit

func applyAssetGenerationActionImpactCloneShape(impact *AssetGenerationActionImpact, cloned *AssetGenerationActionImpact) {
	if impact == nil || cloned == nil {
		return
	}
	cloned.Platforms = append([]string(nil), impact.Platforms...)
	cloned.QualityGrades = append([]string(nil), impact.QualityGrades...)
	cloned.States = append([]string(nil), impact.States...)
}
