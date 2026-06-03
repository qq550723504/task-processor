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
	return buildTaskGenerationActionExecuteRequestHandoffModeRoutingPhase(p.service).run(ctx, taskID, target)
}
