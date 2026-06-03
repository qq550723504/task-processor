package listingkit

import "context"

type taskGenerationActionExecuteRequestHandoffModeRoutingPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationActionExecuteRequestHandoffModeRoutingPhase(service *taskGenerationService) *taskGenerationActionExecuteRequestHandoffModeRoutingPhase {
	return &taskGenerationActionExecuteRequestHandoffModeRoutingPhase{service: service}
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
