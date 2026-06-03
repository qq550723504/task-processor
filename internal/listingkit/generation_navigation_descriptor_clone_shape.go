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
	applyGenerationNavigationDescriptorFollowUpReadCloneRouting(descriptor, cloned)
}
