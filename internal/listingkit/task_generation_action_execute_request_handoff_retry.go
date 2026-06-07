package listingkit

import "context"

type taskGenerationActionExecuteRequestHandoffRetryPhase struct {
	service *taskGenerationService
	request *taskGenerationActionExecuteRequestHandoffRetryRequestPhase
}

type taskGenerationActionExecuteRequestHandoffRetryRequestPhase struct{}

type taskGenerationActionExecuteRequestHandoffRetryResultPhase struct {
	dispatch *taskGenerationActionExecuteRequestHandoffResultDispatchPhase
}

func buildTaskGenerationActionExecuteRequestHandoffRetryPhase(service *taskGenerationService) *taskGenerationActionExecuteRequestHandoffRetryPhase {
	return &taskGenerationActionExecuteRequestHandoffRetryPhase{
		service: service,
		request: buildTaskGenerationActionExecuteRequestHandoffRetryRequestPhase(),
	}
}

func buildTaskGenerationActionExecuteRequestHandoffRetryRequestPhase() *taskGenerationActionExecuteRequestHandoffRetryRequestPhase {
	return &taskGenerationActionExecuteRequestHandoffRetryRequestPhase{}
}

func buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase() *taskGenerationActionExecuteRequestHandoffRetryResultPhase {
	return &taskGenerationActionExecuteRequestHandoffRetryResultPhase{
		dispatch: buildTaskGenerationActionExecuteRequestHandoffResultDispatchPhase(),
	}
}

func (p *taskGenerationActionExecuteRequestHandoffRetryPhase) run(ctx context.Context, taskID string, target *AssetGenerationActionTarget) (*GenerationTaskPage, error) {
	return p.service.RetryTaskGenerationTasks(ctx, taskID, p.request.run(target))
}

func (p *taskGenerationActionExecuteRequestHandoffRetryRequestPhase) run(target *AssetGenerationActionTarget) *RetryGenerationTasksRequest {
	if target == nil {
		return nil
	}
	return cloneRetryGenerationTasksRequest(target.RetryRequest)
}

func (p *taskGenerationActionExecuteRequestHandoffRetryResultPhase) run(retryPage *GenerationTaskPage) *taskGenerationActionExecuteRequestHandoff {
	return p.dispatch.fromRetryPage(retryPage)
}
