package listingkit

func applyGenerationNavigationDescriptorResidualCloneShape(descriptor *GenerationNavigationDescriptor, cloned *GenerationNavigationDescriptor) {
	if descriptor == nil || cloned == nil {
		return
	}
	applyGenerationNavigationDescriptorResidualClonePairing(descriptor, cloned)
	applyGenerationNavigationDescriptorDispatchPlanCloneDelegation(descriptor, cloned)
}
