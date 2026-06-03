package listingkit

type generationNavigationDescriptorCloneShapePhase struct{}

func buildGenerationNavigationDescriptorCloneShapePhase() *generationNavigationDescriptorCloneShapePhase {
	return &generationNavigationDescriptorCloneShapePhase{}
}

func (p *generationNavigationDescriptorCloneShapePhase) run(descriptor *GenerationNavigationDescriptor, cloned *GenerationNavigationDescriptor) {
	if descriptor == nil || cloned == nil {
		return
	}
	cloned.Conditional = cloneGenerationConditionalState(descriptor.Conditional)
	cloned.DispatchPlan = cloneGenerationNavigationDispatchPlan(descriptor.DispatchPlan)
	if len(descriptor.Invalidates) > 0 {
		cloned.Invalidates = append([]string(nil), descriptor.Invalidates...)
	}
	if len(descriptor.FollowUpReads) > 0 {
		cloned.FollowUpReads = make([]GenerationNavigationFollowUpRead, 0, len(descriptor.FollowUpReads))
		for _, item := range descriptor.FollowUpReads {
			cloned.FollowUpReads = append(cloned.FollowUpReads, cloneGenerationNavigationFollowUpRead(item))
		}
	}
}
