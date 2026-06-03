package listingkit

func applyGenerationNavigationDescriptorResidualCloneShape(descriptor *GenerationNavigationDescriptor, cloned *GenerationNavigationDescriptor) {
	if descriptor == nil || cloned == nil {
		return
	}
	cloned.Conditional = cloneGenerationConditionalState(descriptor.Conditional)
	cloned.DispatchPlan = cloneGenerationNavigationDispatchPlan(descriptor.DispatchPlan)
	if len(descriptor.Invalidates) > 0 {
		cloned.Invalidates = append([]string(nil), descriptor.Invalidates...)
	}
}
