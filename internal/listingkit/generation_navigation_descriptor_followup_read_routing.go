package listingkit

func applyGenerationNavigationDescriptorFollowUpReadCloneRouting(descriptor *GenerationNavigationDescriptor, cloned *GenerationNavigationDescriptor) {
	if descriptor == nil || cloned == nil {
		return
	}
	if len(descriptor.FollowUpReads) == 0 {
		return
	}
	cloned.FollowUpReads = make([]GenerationNavigationFollowUpRead, 0, len(descriptor.FollowUpReads))
	for _, item := range descriptor.FollowUpReads {
		cloned.FollowUpReads = append(cloned.FollowUpReads, cloneGenerationNavigationFollowUpRead(item))
	}
}
