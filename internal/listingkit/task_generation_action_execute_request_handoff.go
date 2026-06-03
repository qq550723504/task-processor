package listingkit

import "context"

type taskGenerationActionExecuteRequestHandoffPhase struct {
	service *taskGenerationService
}

type taskGenerationActionExecuteRequestHandoff struct {
	retryPage        *GenerationTaskPage
	queuePage        *GenerationQueuePage
	persistenceQueue *GenerationWorkQueue
}

func buildTaskGenerationActionExecuteRequestHandoffPhase(service *taskGenerationService) *taskGenerationActionExecuteRequestHandoffPhase {
	return &taskGenerationActionExecuteRequestHandoffPhase{service: service}
}

func (p *taskGenerationActionExecuteRequestHandoffPhase) run(ctx context.Context, taskID string, target *AssetGenerationActionTarget) (*taskGenerationActionExecuteRequestHandoff, error) {
	switch target.InteractionMode {
	case "retryable":
		retryPage, err := p.service.RetryTaskGenerationTasks(ctx, taskID, cloneRetryGenerationTasksRequest(target.RetryRequest))
		if err != nil {
			return nil, err
		}
		return &taskGenerationActionExecuteRequestHandoff{
			retryPage:        retryPage,
			persistenceQueue: generationWorkQueueFromRetryPage(retryPage),
		}, nil
	default:
		queuePage, err := p.service.GetTaskGenerationQueue(ctx, taskID, cloneGenerationQueueQuery(target.QueueQuery))
		if err != nil {
			return nil, err
		}
		return &taskGenerationActionExecuteRequestHandoff{
			queuePage:        queuePage,
			persistenceQueue: generationWorkQueueFromPage(queuePage),
		}, nil
	}
}
