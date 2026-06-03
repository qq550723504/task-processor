package listingkit

func applyGenerationNavigationDescriptorCloneShapePairing(descriptor *GenerationNavigationDescriptor, cloned *GenerationNavigationDescriptor) {
	if descriptor == nil || cloned == nil {
		return
	}
	applyGenerationNavigationDescriptorResidualCloneShape(descriptor, cloned)
	applyGenerationNavigationDescriptorFollowUpReadCloneRouting(descriptor, cloned)
}
