package listingkit

type taskGenerationActionProjectionPhase struct{}

type taskGenerationActionProjectionInput struct {
	actionKey             string
	target                *AssetGenerationActionTarget
	responseMode          string
	previousReviewSession *GenerationReviewSession
	currentResult         *ListingKitResult
	refresh               *taskGenerationActionRefreshResult
	execution             *taskGenerationActionExecution
}

func buildTaskGenerationActionProjectionPhase() *taskGenerationActionProjectionPhase {
	return &taskGenerationActionProjectionPhase{}
}

func (p *taskGenerationActionProjectionPhase) run(input *taskGenerationActionProjectionInput) *GenerationActionExecutionResult {
	if input == nil {
		return &GenerationActionExecutionResult{}
	}

	result := &GenerationActionExecutionResult{
		ActionKey:    input.actionKey,
		ResponseMode: normalizeGenerationActionResponseMode(input.responseMode),
	}

	if input.target != nil {
		result.InteractionMode = input.target.InteractionMode
		result.ResolvedTarget = input.target
	}

	if input.execution != nil {
		result.Retry = input.execution.retryPage
		result.Queue = input.execution.queuePage
	}

	if input.refresh != nil {
		result.Overview = input.refresh.overview
		result.PlatformRenderPreviews = input.refresh.platformRenderPreviews
	}

	session := buildTaskGenerationActionProjectionSessionPhase().run(input)
	result.ReviewSession = session.reviewSession
	result.ReviewWorkflow = buildGenerationReviewWorkflowResult(input.actionKey, input.target)
	applyGenerationReviewWorkflow(result.ReviewSession, result.ReviewWorkflow)
	result.ReviewPatch = buildGenerationReviewSessionPatch(input.previousReviewSession, result.ReviewSession)
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
