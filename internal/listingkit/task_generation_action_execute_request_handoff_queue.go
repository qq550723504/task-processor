package listingkit

import "context"

type taskGenerationActionExecuteRequestHandoffQueuePhase struct {
	service *taskGenerationService
}

func buildTaskGenerationActionExecuteRequestHandoffQueuePhase(service *taskGenerationService) *taskGenerationActionExecuteRequestHandoffQueuePhase {
	return &taskGenerationActionExecuteRequestHandoffQueuePhase{service: service}
}

func (p *taskGenerationActionExecuteRequestHandoffQueuePhase) run(ctx context.Context, taskID string, target *AssetGenerationActionTarget) (*GenerationQueuePage, error) {
	return p.service.GetTaskGenerationQueue(ctx, taskID, cloneGenerationQueueQuery(target.QueueQuery))
}
