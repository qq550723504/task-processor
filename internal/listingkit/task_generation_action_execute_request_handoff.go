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
	switch target.InteractionMode {
	case "retryable":
		retryPage, err := buildTaskGenerationActionExecuteRequestHandoffRetryPhase(p.service).run(ctx, taskID, target)
		if err != nil {
			return nil, err
		}
		return buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase().run(retryPage), nil
	default:
		queuePage, err := buildTaskGenerationActionExecuteRequestHandoffQueuePhase(p.service).run(ctx, taskID, target)
		if err != nil {
			return nil, err
		}
		return buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase().run(queuePage), nil
	}
}
