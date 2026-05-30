package listingkit

type taskGenerationActionProjectionFinalizePhase struct{}

func buildTaskGenerationActionProjectionFinalizePhase() *taskGenerationActionProjectionFinalizePhase {
	return &taskGenerationActionProjectionFinalizePhase{}
}

func (p *taskGenerationActionProjectionFinalizePhase) run(
	input *taskGenerationActionProjectionInput,
	result *GenerationActionExecutionResult,
	reviewSession *GenerationReviewSession,
) *GenerationActionExecutionResult {
	if result == nil {
		result = &GenerationActionExecutionResult{}
	}

	result.ReviewSession = nil
	if reviewSession != nil {
		result.ReviewSession = reviewSession
	}

	actionKey := ""
	var target *AssetGenerationActionTarget
	var previousReviewSession *GenerationReviewSession
	if input != nil {
		actionKey = input.actionKey
		target = input.target
		previousReviewSession = input.previousReviewSession
	}

	result.ReviewWorkflow = buildGenerationReviewWorkflowResult(actionKey, target)
	applyGenerationReviewWorkflow(result.ReviewSession, result.ReviewWorkflow)
	result.ReviewPatch = buildGenerationReviewSessionPatch(previousReviewSession, result.ReviewSession)
	if result.ReviewPatch != nil {
		result.ReviewPatch.LastWorkflowResult = result.ReviewWorkflow
		result.DeltaToken = result.ReviewPatch.DeltaToken
	}
	if result.DeltaToken == "" {
		result.DeltaToken = buildGenerationReviewDeltaToken(result.ReviewSession)
	}
	if result.ResponseMode == "patch_only" {
		result.ReviewSession = nil
		result.PlatformRenderPreviews = nil
	}

	return result
}
