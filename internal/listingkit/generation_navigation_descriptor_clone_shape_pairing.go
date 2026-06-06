package listingkit

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
