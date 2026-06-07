package listingkit

import "context"

type taskGenerationActionExecuteRequestHandoffQueuePhase struct {
	service *taskGenerationService
	request *taskGenerationActionExecuteRequestHandoffQueueRequestPhase
}

type taskGenerationActionExecuteRequestHandoffQueueRequestPhase struct{}

type taskGenerationActionExecuteRequestHandoffQueueResultPhase struct {
	dispatch *taskGenerationActionExecuteRequestHandoffResultDispatchPhase
}

func buildTaskGenerationActionExecuteRequestHandoffQueuePhase(service *taskGenerationService) *taskGenerationActionExecuteRequestHandoffQueuePhase {
	return &taskGenerationActionExecuteRequestHandoffQueuePhase{
		service: service,
		request: buildTaskGenerationActionExecuteRequestHandoffQueueRequestPhase(),
	}
}

func buildTaskGenerationActionExecuteRequestHandoffQueueRequestPhase() *taskGenerationActionExecuteRequestHandoffQueueRequestPhase {
	return &taskGenerationActionExecuteRequestHandoffQueueRequestPhase{}
}

func buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase() *taskGenerationActionExecuteRequestHandoffQueueResultPhase {
	return &taskGenerationActionExecuteRequestHandoffQueueResultPhase{
		dispatch: buildTaskGenerationActionExecuteRequestHandoffResultDispatchPhase(),
	}
}

func (p *taskGenerationActionExecuteRequestHandoffQueuePhase) run(ctx context.Context, taskID string, target *AssetGenerationActionTarget) (*GenerationQueuePage, error) {
	return p.service.GetTaskGenerationQueue(ctx, taskID, p.request.run(target))
}

func (p *taskGenerationActionExecuteRequestHandoffQueueRequestPhase) run(target *AssetGenerationActionTarget) *GenerationQueueQuery {
	if target == nil {
		return nil
	}
	return cloneGenerationQueueQuery(target.QueueQuery)
}

func (p *taskGenerationActionExecuteRequestHandoffQueueResultPhase) run(queuePage *GenerationQueuePage) *taskGenerationActionExecuteRequestHandoff {
	return p.dispatch.fromQueuePage(queuePage)
}
