package listingkit

func applyIdentityToNavigationTarget(target *GenerationReviewNavigationTarget) *GenerationReviewNavigationTarget {
	if target == nil {
		return nil
	}
	target.ResourceKind = buildGenerationNavigationTargetResourceKind(target)
	target.CacheKey = buildGenerationNavigationTargetCacheKey(target)
	target.CachePolicy = buildGenerationNavigationTargetCachePolicy(target)
	target.RevalidateAfterAction = generationNavigationTargetRevalidateAfterAction(target)
	target.Descriptor = buildGenerationNavigationDescriptor(target)
	return target
}

func cloneGenerationNavigationDescriptor(descriptor *GenerationNavigationDescriptor) *GenerationNavigationDescriptor {
	if descriptor == nil {
		return nil
	}
	cloned := *descriptor
	applyGenerationNavigationDescriptorCloneShapePairing(descriptor, &cloned)
	return &cloned
}

func cloneGenerationNavigationDispatchPlan(plan *GenerationNavigationDispatchPlan) *GenerationNavigationDispatchPlan {
	if plan == nil {
		return nil
	}
	cloned := *plan
	if len(plan.Steps) > 0 {
		cloned.Steps = make([]GenerationNavigationDispatchStep, 0, len(plan.Steps))
		for _, step := range plan.Steps {
			cloned.Steps = append(cloned.Steps, cloneGenerationNavigationDispatchPlanStep(step))
		}
	}
	return &cloned
}

func applyGenerationNavigationDescriptorCloneShapePairing(descriptor *GenerationNavigationDescriptor, cloned *GenerationNavigationDescriptor) {
	if descriptor == nil || cloned == nil {
		return
	}
	cloned.Conditional = cloneGenerationConditionalState(descriptor.Conditional)
	if len(descriptor.Invalidates) > 0 {
		cloned.Invalidates = append([]string(nil), descriptor.Invalidates...)
	}
	cloned.DispatchPlan = cloneGenerationNavigationDispatchPlan(descriptor.DispatchPlan)
	cloned.FollowUpReads = cloneGenerationNavigationFollowUpReadSlice(descriptor.FollowUpReads)
}

func cloneGenerationNavigationFollowUpReadSlice(items []GenerationNavigationFollowUpRead) []GenerationNavigationFollowUpRead {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]GenerationNavigationFollowUpRead, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, cloneGenerationNavigationFollowUpRead(item))
	}
	return cloned
}

func cloneGenerationNavigationFollowUpRead(item GenerationNavigationFollowUpRead) GenerationNavigationFollowUpRead {
	cloned := item
	applyGenerationNavigationFollowUpReadCloneShape(item, &cloned)
	return cloned
}

func applyGenerationNavigationFollowUpReadCloneShape(item GenerationNavigationFollowUpRead, cloned *GenerationNavigationFollowUpRead) {
	if cloned == nil {
		return
	}
	cloned.Query = cloneGenerationQueueQuery(item.Query)
}

func cloneGenerationNavigationDispatchPlanStep(step GenerationNavigationDispatchStep) GenerationNavigationDispatchStep {
	return GenerationNavigationDispatchStep{
		Kind:               step.Kind,
		ResponseMode:       step.ResponseMode,
		CachePreference:    step.CachePreference,
		RequiresRevalidate: step.RequiresRevalidate,
		Query:              cloneGenerationQueueQuery(step.Query),
	}
}
