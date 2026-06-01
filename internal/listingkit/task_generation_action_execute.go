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
	execution := &taskGenerationActionExecution{}

	switch target.InteractionMode {
	case "retryable":
		retryPage, err := p.service.RetryTaskGenerationTasks(ctx, taskID, cloneRetryGenerationTasksRequest(target.RetryRequest))
		if err != nil {
			return nil, err
		}
		execution.retryPage = retryPage
		execution.persistenceSession = buildGenerationReviewSession(baseResult, generationWorkQueueFromRetryPage(retryPage), target.QueueQuery)
	default:
		queuePage, err := p.service.GetTaskGenerationQueue(ctx, taskID, cloneGenerationQueueQuery(target.QueueQuery))
		if err != nil {
			return nil, err
		}
		execution.queuePage = queuePage
		execution.persistenceSession = buildGenerationReviewSession(baseResult, generationWorkQueueFromPage(queuePage), target.QueueQuery)
	}

	return execution, nil
}
