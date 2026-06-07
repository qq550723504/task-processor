package listingkit

func cloneAssetGenerationActionTarget(target *AssetGenerationActionTarget) *AssetGenerationActionTarget {
	if target == nil {
		return nil
	}
	cloned := *target
	cloned.Filters = cloneAssetGenerationFilters(target.Filters)
	cloned.QueueQuery = cloneGenerationQueueQuery(target.QueueQuery)
	cloned.RetryRequest = cloneRetryGenerationTasksRequest(target.RetryRequest)
	cloned.ExpectedImpact = cloneAssetGenerationActionImpact(target.ExpectedImpact)
	cloned.NavigationTarget = cloneGenerationReviewNavigationTarget(target.NavigationTarget)
	return &cloned
}

func cloneAssetGenerationActionImpact(impact *AssetGenerationActionImpact) *AssetGenerationActionImpact {
	if impact == nil {
		return nil
	}
	cloned := *impact
	applyAssetGenerationActionImpactPlatformsClone(impact, &cloned)
	applyAssetGenerationActionImpactQualityGradesClone(impact, &cloned)
	applyAssetGenerationActionImpactStatesClone(impact, &cloned)
	return &cloned
}
