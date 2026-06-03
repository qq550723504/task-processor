package listingkit

import "context"

type taskGenerationActionExecuteRequestHandoffRetryPhase struct {
	service *taskGenerationService
	request *taskGenerationActionExecuteRequestHandoffRetryRequestPhase
}

func buildTaskGenerationActionExecuteRequestHandoffRetryPhase(service *taskGenerationService) *taskGenerationActionExecuteRequestHandoffRetryPhase {
	return &taskGenerationActionExecuteRequestHandoffRetryPhase{
		service: service,
		request: buildTaskGenerationActionExecuteRequestHandoffRetryRequestPhase(),
	}
}

func (p *taskGenerationActionExecuteRequestHandoffRetryPhase) run(ctx context.Context, taskID string, target *AssetGenerationActionTarget) (*GenerationTaskPage, error) {
	return p.service.RetryTaskGenerationTasks(ctx, taskID, p.request.run(target))
}
