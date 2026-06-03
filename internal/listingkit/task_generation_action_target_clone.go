package listingkit

func cloneAssetGenerationActionTarget(target *AssetGenerationActionTarget) *AssetGenerationActionTarget {
	if target == nil {
		return nil
	}
	cloned := *target
	buildTaskGenerationActionTargetCloneShapePhase().run(target, &cloned)
	return &cloned
}

func cloneAssetGenerationActionImpact(impact *AssetGenerationActionImpact) *AssetGenerationActionImpact {
	if impact == nil {
		return nil
	}
	cloned := *impact
	cloned.Platforms = append([]string(nil), impact.Platforms...)
	cloned.QualityGrades = append([]string(nil), impact.QualityGrades...)
	cloned.States = append([]string(nil), impact.States...)
	return &cloned
}
