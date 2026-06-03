package listingkit

import "context"

type taskGenerationActionExecuteRequestHandoffRetryPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationActionExecuteRequestHandoffRetryPhase(service *taskGenerationService) *taskGenerationActionExecuteRequestHandoffRetryPhase {
	return &taskGenerationActionExecuteRequestHandoffRetryPhase{service: service}
}

func (p *taskGenerationActionExecuteRequestHandoffRetryPhase) run(ctx context.Context, taskID string, target *AssetGenerationActionTarget) (*GenerationTaskPage, error) {
	return p.service.RetryTaskGenerationTasks(ctx, taskID, cloneRetryGenerationTasksRequest(target.RetryRequest))
}
