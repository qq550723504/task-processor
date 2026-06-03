package listingkit

type generationNavigationDispatchPlanCloneShapePhase struct{}

func buildGenerationNavigationDispatchPlanCloneShapePhase() *generationNavigationDispatchPlanCloneShapePhase {
	return &generationNavigationDispatchPlanCloneShapePhase{}
}

func (p *generationNavigationDispatchPlanCloneShapePhase) run(plan *GenerationNavigationDispatchPlan, cloned *GenerationNavigationDispatchPlan) {
	if plan == nil || cloned == nil {
		return
	}
	if len(plan.Steps) == 0 {
		return
	}
	cloned.Steps = make([]GenerationNavigationDispatchStep, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		cloned.Steps = append(cloned.Steps, cloneGenerationNavigationDispatchPlanStep(step))
	}
}
