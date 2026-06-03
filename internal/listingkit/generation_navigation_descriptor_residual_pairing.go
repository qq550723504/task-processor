package listingkit

func applyGenerationNavigationDescriptorResidualClonePairing(descriptor *GenerationNavigationDescriptor, cloned *GenerationNavigationDescriptor) {
	if descriptor == nil || cloned == nil {
		return
	}
	cloned.Conditional = cloneGenerationConditionalState(descriptor.Conditional)
	if len(descriptor.Invalidates) > 0 {
		cloned.Invalidates = append([]string(nil), descriptor.Invalidates...)
	}
}
