package listingkit

func cloneGenerationNavigationDispatchPlanStep(step GenerationNavigationDispatchStep) GenerationNavigationDispatchStep {
	return GenerationNavigationDispatchStep{
		Kind:               step.Kind,
		ResponseMode:       step.ResponseMode,
		CachePreference:    step.CachePreference,
		RequiresRevalidate: step.RequiresRevalidate,
		Query:              cloneGenerationQueueQuery(step.Query),
	}
}
