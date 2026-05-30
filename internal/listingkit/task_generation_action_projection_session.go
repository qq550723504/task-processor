package listingkit

type taskGenerationActionProjectionSessionPhase struct{}

type taskGenerationActionProjectionSessionResult struct {
	currentResult *ListingKitResult
	reviewQueue   *GenerationWorkQueue
	reviewSession *GenerationReviewSession
}

func buildTaskGenerationActionProjectionSessionPhase() *taskGenerationActionProjectionSessionPhase {
	return &taskGenerationActionProjectionSessionPhase{}
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
