package listingkit

import listinggeneration "task-processor/internal/listingkit/generation"

type taskGenerationActionProjectionPhase struct{}

type taskGenerationActionProjectionFinalizePhase struct{}

type taskGenerationActionProjectionSessionPhase struct{}

type taskGenerationActionProjectionSessionResult struct {
	currentResult *ListingKitResult
	reviewQueue   *GenerationWorkQueue
	reviewSession *GenerationReviewSession
}

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

func buildTaskGenerationActionProjectionFinalizePhase() *taskGenerationActionProjectionFinalizePhase {
	return &taskGenerationActionProjectionFinalizePhase{}
}

func buildTaskGenerationActionProjectionSessionPhase() *taskGenerationActionProjectionSessionPhase {
	return &taskGenerationActionProjectionSessionPhase{}
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

func (p *taskGenerationActionProjectionSessionPhase) run(input *taskGenerationActionProjectionInput) *taskGenerationActionProjectionSessionResult {
	if input == nil {
		return &taskGenerationActionProjectionSessionResult{}
	}

	currentResult := input.currentResult
	if input.refresh != nil && input.refresh.currentResult != nil {
		currentResult = input.refresh.currentResult
	}

	reviewQueue := taskGenerationActionProjectionReviewQueue(input)

	return &taskGenerationActionProjectionSessionResult{
		currentResult: currentResult,
		reviewQueue:   reviewQueue,
		reviewSession: buildGenerationReviewSession(currentResult, reviewQueue, projectionQueueQuery(input.target)),
	}
}

func taskGenerationActionProjectionReviewQueue(input *taskGenerationActionProjectionInput) *GenerationWorkQueue {
	if input == nil || input.target == nil {
		return nil
	}
	if input.execution == nil {
		return nil
	}
	switch input.target.InteractionMode {
	case "retryable":
		return generationWorkQueueFromRetryPage(input.execution.retryPage)
	default:
		return generationWorkQueueFromPage(input.execution.queuePage)
	}
}

func projectionQueueQuery(target *AssetGenerationActionTarget) *GenerationQueueQuery {
	if target == nil {
		return nil
	}
	return target.QueueQuery
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

func buildGenerationReviewWorkflowResult(actionKey string, target *AssetGenerationActionTarget) *GenerationReviewWorkflowResult {
	if target == nil {
		return nil
	}
	workflow := listinggeneration.BuildReviewWorkflowResult(actionKey)
	query := target.QueueQuery
	result := &GenerationReviewWorkflowResult{
		ActionKey: workflow.ActionKey,
		Status:    workflow.Status,
		Message:   workflow.Message,
	}
	if query != nil {
		result.Platform = query.Platform
		result.Slot = query.Slot
		result.Capability = query.PreviewCapability
	}
	return result
}

func applyGenerationReviewWorkflow(session *GenerationReviewSession, workflow *GenerationReviewWorkflowResult) {
	if session == nil || workflow == nil {
		return
	}
	session.LastWorkflowResult = workflow
	for i := range session.Sections {
		if workflow.Capability != "" && session.Sections[i].Capability != workflow.Capability {
			continue
		}
		session.Sections[i].WorkflowState = listinggeneration.ReviewWorkflowState(workflow.ActionKey)
		session.Sections[i].WorkflowMessage = workflow.Message
	}
}
