package listingkit

type taskGenerationNavigationDispatchProjectionPhase struct{}

func buildTaskGenerationNavigationDispatchProjectionPhase() *taskGenerationNavigationDispatchProjectionPhase {
	return &taskGenerationNavigationDispatchProjectionPhase{}
}

func (p *taskGenerationNavigationDispatchProjectionPhase) run(response *GenerationReviewNavigationDispatchResponse, planMode string, executedPlan *GenerationNavigationDispatchExecution) *GenerationReviewNavigationDispatchResponse {
	if response == nil {
		return nil
	}

	response.PlanMode = planMode
	if planMode == "execute_plan" && executedPlan != nil {
		applyExecutedPlanToDispatchResponse(response, executedPlan)
	}

	return finalizeGenerationReviewNavigationDispatchResponse(response)
}
