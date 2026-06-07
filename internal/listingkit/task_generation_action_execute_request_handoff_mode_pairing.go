package listingkit

import "context"

type taskGenerationActionExecuteRequestHandoffModePairingPhase struct {
	service       *taskGenerationService
	normalization *taskGenerationActionExecuteRequestHandoffModePairingNormalizationPhase
}

type taskGenerationActionExecuteRequestHandoffModePairingNormalizationPhase struct {
	retryResult *taskGenerationActionExecuteRequestHandoffRetryResultPhase
	queueResult *taskGenerationActionExecuteRequestHandoffQueueResultPhase
}

func buildTaskGenerationActionExecuteRequestHandoffModePairingPhase(service *taskGenerationService) *taskGenerationActionExecuteRequestHandoffModePairingPhase {
	return &taskGenerationActionExecuteRequestHandoffModePairingPhase{
		service:       service,
		normalization: buildTaskGenerationActionExecuteRequestHandoffModePairingNormalizationPhase(),
	}
}

func buildTaskGenerationActionExecuteRequestHandoffModePairingNormalizationPhase() *taskGenerationActionExecuteRequestHandoffModePairingNormalizationPhase {
	return &taskGenerationActionExecuteRequestHandoffModePairingNormalizationPhase{
		retryResult: buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase(),
		queueResult: buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase(),
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

func (p *taskGenerationActionExecuteRequestHandoffModePairingNormalizationPhase) fromRetryPage(retryPage *GenerationTaskPage) *taskGenerationActionExecuteRequestHandoff {
	return p.retryResult.run(retryPage)
}

func (p *taskGenerationActionExecuteRequestHandoffModePairingNormalizationPhase) fromQueuePage(queuePage *GenerationQueuePage) *taskGenerationActionExecuteRequestHandoff {
	return p.queueResult.run(queuePage)
}
