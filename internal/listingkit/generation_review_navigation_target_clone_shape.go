package listingkit

func applyGenerationReviewNavigationTargetCloneShape(target *GenerationReviewNavigationTarget, cloned *GenerationReviewNavigationTarget) {
	if target == nil || cloned == nil {
		return
	}
	cloned.Conditional = cloneGenerationConditionalState(target.Conditional)
	cloned.Descriptor = cloneGenerationNavigationDescriptor(target.Descriptor)
	cloned.QueueQuery = cloneGenerationQueueQuery(target.QueueQuery)
	cloned.SessionQuery = cloneGenerationQueueQuery(target.SessionQuery)
	cloned.PreviewQuery = cloneGenerationQueueQuery(target.PreviewQuery)
	cloned.ActionTarget = cloneAssetGenerationActionTarget(target.ActionTarget)
}
