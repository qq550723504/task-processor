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
	applyAssetGenerationActionImpactCloneShape(impact, &cloned)
	return &cloned
}
