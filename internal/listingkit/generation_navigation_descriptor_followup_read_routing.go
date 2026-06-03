package listingkit

func applyGenerationNavigationDescriptorFollowUpReadCloneRouting(descriptor *GenerationNavigationDescriptor, cloned *GenerationNavigationDescriptor) {
	if descriptor == nil || cloned == nil {
		return
	}
	applyGenerationNavigationDescriptorFollowUpReadRoutingPairing(descriptor, cloned)
}
