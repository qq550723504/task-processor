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
	var reviewSession *GenerationReviewSession
	if session != nil {
		reviewSession = session.reviewSession
	}

	return buildTaskGenerationActionProjectionFinalizePhase().run(input, result, reviewSession)
}
