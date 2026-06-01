package listingkit

type taskGenerationActionFinalizePhase struct{}

func buildTaskGenerationActionFinalizePhase() *taskGenerationActionFinalizePhase {
	return &taskGenerationActionFinalizePhase{}
}

func (p *taskGenerationActionFinalizePhase) run(
	result *GenerationActionExecutionResult,
	projection *GenerationActionExecutionResult,
) *GenerationActionExecutionResult {
	if result == nil {
		result = &GenerationActionExecutionResult{}
	}
	if projection != nil {
		result.Overview = projection.Overview
		result.Queue = projection.Queue
		result.Retry = projection.Retry
		result.ReviewWorkflow = projection.ReviewWorkflow
		result.ReviewSession = projection.ReviewSession
		result.ReviewPatch = projection.ReviewPatch
		result.PlatformRenderPreviews = projection.PlatformRenderPreviews
		result.DeltaToken = projection.DeltaToken
	}
	return applyGenerationConditionalStateToActionResult(result)
}
