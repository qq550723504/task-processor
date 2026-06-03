package listingkit

import "context"

type taskGenerationActionExecuteRequestHandoffModePairingPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationActionExecuteRequestHandoffModePairingPhase(service *taskGenerationService) *taskGenerationActionExecuteRequestHandoffModePairingPhase {
	return &taskGenerationActionExecuteRequestHandoffModePairingPhase{service: service}
}

func (p *taskGenerationActionExecuteRequestHandoffModePairingPhase) runRetryable(ctx context.Context, taskID string, target *AssetGenerationActionTarget) (*taskGenerationActionExecuteRequestHandoff, error) {
	retryPage, err := buildTaskGenerationActionExecuteRequestHandoffRetryPhase(p.service).run(ctx, taskID, target)
	if err != nil {
		return nil, err
	}
	return buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase().run(retryPage), nil
}

func (p *taskGenerationActionExecuteRequestHandoffModePairingPhase) runQueue(ctx context.Context, taskID string, target *AssetGenerationActionTarget) (*taskGenerationActionExecuteRequestHandoff, error) {
	queuePage, err := buildTaskGenerationActionExecuteRequestHandoffQueuePhase(p.service).run(ctx, taskID, target)
	if err != nil {
		return nil, err
	}
	return buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase().run(queuePage), nil
}
