package listingkit

type taskGenerationActionTargetCloneShapePhase struct{}

func buildTaskGenerationActionTargetCloneShapePhase() *taskGenerationActionTargetCloneShapePhase {
	return &taskGenerationActionTargetCloneShapePhase{}
}

func (p *taskGenerationActionTargetCloneShapePhase) run(target *AssetGenerationActionTarget, cloned *AssetGenerationActionTarget) {
	if target == nil || cloned == nil {
		return
	}
	cloned.Filters = cloneAssetGenerationFilters(target.Filters)
	cloned.QueueQuery = cloneGenerationQueueQuery(target.QueueQuery)
	cloned.RetryRequest = cloneRetryGenerationTasksRequest(target.RetryRequest)
	cloned.ExpectedImpact = cloneAssetGenerationActionImpact(target.ExpectedImpact)
	cloned.NavigationTarget = cloneGenerationReviewNavigationTarget(target.NavigationTarget)
}
