package listingkit

import "context"

type taskGenerationActionExecuteRequestHandoffModePairingPhase struct {
	service       *taskGenerationService
	normalization *taskGenerationActionExecuteRequestHandoffModePairingNormalizationPhase
}

func buildTaskGenerationActionExecuteRequestHandoffModePairingPhase(service *taskGenerationService) *taskGenerationActionExecuteRequestHandoffModePairingPhase {
	return &taskGenerationActionExecuteRequestHandoffModePairingPhase{
		service:       service,
		normalization: buildTaskGenerationActionExecuteRequestHandoffModePairingNormalizationPhase(),
	}
}

func (p *taskGenerationActionExecuteRequestHandoffModePairingPhase) runRetryable(ctx context.Context, taskID string, target *AssetGenerationActionTarget) (*taskGenerationActionExecuteRequestHandoff, error) {
	retryPage, err := buildTaskGenerationActionExecuteRequestHandoffRetryPhase(p.service).run(ctx, taskID, target)
	if err != nil {
		return nil, err
	}
	return p.normalization.fromRetryPage(retryPage), nil
}

func (p *taskGenerationActionExecuteRequestHandoffModePairingPhase) runQueue(ctx context.Context, taskID string, target *AssetGenerationActionTarget) (*taskGenerationActionExecuteRequestHandoff, error) {
	queuePage, err := buildTaskGenerationActionExecuteRequestHandoffQueuePhase(p.service).run(ctx, taskID, target)
	if err != nil {
		return nil, err
	}
	return p.normalization.fromQueuePage(queuePage), nil
}
