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
	cloned.Platforms = append([]string(nil), impact.Platforms...)
	cloned.QualityGrades = append([]string(nil), impact.QualityGrades...)
	cloned.States = append([]string(nil), impact.States...)
	return &cloned
}
