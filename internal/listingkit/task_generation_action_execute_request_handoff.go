package listingkit

import "context"

type taskGenerationActionExecuteRequestHandoffPhase struct {
	service *taskGenerationService
}

type taskGenerationActionExecuteRequestHandoffModeRoutingPhase struct {
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

func buildTaskGenerationActionExecuteRequestHandoffModeRoutingPhase(service *taskGenerationService) *taskGenerationActionExecuteRequestHandoffModeRoutingPhase {
	return &taskGenerationActionExecuteRequestHandoffModeRoutingPhase{service: service}
}

func (p *taskGenerationActionExecuteRequestHandoffPhase) run(ctx context.Context, taskID string, target *AssetGenerationActionTarget) (*taskGenerationActionExecuteRequestHandoff, error) {
	return buildTaskGenerationActionExecuteRequestHandoffModeRoutingPhase(p.service).run(ctx, taskID, target)
}

func (p *taskGenerationActionExecuteRequestHandoffModeRoutingPhase) run(ctx context.Context, taskID string, target *AssetGenerationActionTarget) (*taskGenerationActionExecuteRequestHandoff, error) {
	pairing := buildTaskGenerationActionExecuteRequestHandoffModePairingPhase(p.service)

	switch target.InteractionMode {
	case "retryable":
		return pairing.runRetryable(ctx, taskID, target)
	default:
		return pairing.runQueue(ctx, taskID, target)
	}
}
