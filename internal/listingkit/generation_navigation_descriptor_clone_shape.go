package listingkit

type generationNavigationDescriptorCloneShapePhase struct{}

func buildGenerationNavigationDescriptorCloneShapePhase() *generationNavigationDescriptorCloneShapePhase {
	return &generationNavigationDescriptorCloneShapePhase{}
}

func (p *generationNavigationDescriptorCloneShapePhase) run(descriptor *GenerationNavigationDescriptor, cloned *GenerationNavigationDescriptor) {
	if descriptor == nil || cloned == nil {
		return
	}
	applyGenerationNavigationDescriptorResidualCloneShape(descriptor, cloned)
	if len(descriptor.FollowUpReads) > 0 {
		cloned.FollowUpReads = make([]GenerationNavigationFollowUpRead, 0, len(descriptor.FollowUpReads))
		for _, item := range descriptor.FollowUpReads {
			cloned.FollowUpReads = append(cloned.FollowUpReads, cloneGenerationNavigationFollowUpRead(item))
		}
	}
}
