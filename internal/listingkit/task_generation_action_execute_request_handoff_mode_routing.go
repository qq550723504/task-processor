package listingkit

import "context"

type taskGenerationActionExecuteRequestHandoffModeRoutingPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationActionExecuteRequestHandoffModeRoutingPhase(service *taskGenerationService) *taskGenerationActionExecuteRequestHandoffModeRoutingPhase {
	return &taskGenerationActionExecuteRequestHandoffModeRoutingPhase{service: service}
}

func (p *taskGenerationActionExecuteRequestHandoffModeRoutingPhase) run(ctx context.Context, taskID string, target *AssetGenerationActionTarget) (*taskGenerationActionExecuteRequestHandoff, error) {
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
