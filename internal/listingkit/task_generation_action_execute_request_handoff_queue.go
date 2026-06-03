package listingkit

import "context"

type taskGenerationActionExecuteRequestHandoffQueuePhase struct {
	service *taskGenerationService
	request *taskGenerationActionExecuteRequestHandoffQueueRequestPhase
}

func buildTaskGenerationActionExecuteRequestHandoffQueuePhase(service *taskGenerationService) *taskGenerationActionExecuteRequestHandoffQueuePhase {
	return &taskGenerationActionExecuteRequestHandoffQueuePhase{
		service: service,
		request: buildTaskGenerationActionExecuteRequestHandoffQueueRequestPhase(),
	}
}

func (p *taskGenerationActionExecuteRequestHandoffQueuePhase) run(ctx context.Context, taskID string, target *AssetGenerationActionTarget) (*GenerationQueuePage, error) {
	return p.service.GetTaskGenerationQueue(ctx, taskID, p.request.run(target))
}
