package listingkit

import "context"

type taskGenerationActionExecutePhase struct {
	service *taskGenerationService
}

type taskGenerationActionExecution struct {
	retryPage          *GenerationTaskPage
	queuePage          *GenerationQueuePage
	persistenceSession *GenerationReviewSession
}

func buildTaskGenerationActionExecutePhase(service *taskGenerationService) *taskGenerationActionExecutePhase {
	return &taskGenerationActionExecutePhase{service: service}
}

func (p *taskGenerationActionExecutePhase) run(ctx context.Context, taskID string, baseResult *ListingKitResult, target *AssetGenerationActionTarget) (*taskGenerationActionExecution, error) {
	handoff, err := buildTaskGenerationActionExecuteRequestHandoffPhase(p.service).run(ctx, taskID, target)
	if err != nil {
		return nil, err
	}

	return &taskGenerationActionExecution{
		retryPage:          handoff.retryPage,
		queuePage:          handoff.queuePage,
		persistenceSession: buildGenerationReviewSession(baseResult, handoff.persistenceQueue, target.QueueQuery),
	}, nil
}
