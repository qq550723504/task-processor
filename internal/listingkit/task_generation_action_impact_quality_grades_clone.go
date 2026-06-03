package listingkit

func applyAssetGenerationActionImpactQualityGradesClone(impact *AssetGenerationActionImpact, cloned *AssetGenerationActionImpact) {
	if impact == nil || cloned == nil {
		return
	}
	cloned.QualityGrades = append([]string(nil), impact.QualityGrades...)
}
