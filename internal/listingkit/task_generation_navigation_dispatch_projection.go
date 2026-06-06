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

func applyExecutedPlanToDispatchResponse(response *GenerationReviewNavigationDispatchResponse, execution *GenerationNavigationDispatchExecution) {
	applyGenerationNavigationDispatchExecutionMerge(response, execution)
}

func finalizeGenerationReviewNavigationDispatchResponse(response *GenerationReviewNavigationDispatchResponse) *GenerationReviewNavigationDispatchResponse {
	if response == nil {
		return nil
	}
	response.PanelUpdate = buildGenerationReviewPanelUpdateFromDispatch(response)
	if (response.ReviewPreview != nil && response.ReviewPreview.NotModified) ||
		(response.Queue != nil && response.Queue.NotModified) ||
		(response.PanelUpdate != nil && response.PanelUpdate.NoChanges) {
		response.NotModified = true
	}
	return applyGenerationConditionalStateToNavigationDispatchResponse(response)
}
