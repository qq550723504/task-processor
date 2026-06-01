package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type taskGenerationActionTemporalStandardPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationActionTemporalStandardPhase(service *taskGenerationService) *taskGenerationActionTemporalStandardPhase {
	return &taskGenerationActionTemporalStandardPhase{service: service}
}

func (p *taskGenerationActionTemporalStandardPhase) run(
	ctx context.Context,
	taskID string,
	req *ExecuteGenerationActionRequest,
) (*GenerationActionExecutionResult, error) {
	client, enabled := p.service.standardWorkflow()
	if !enabled || client == nil {
		return nil, fmt.Errorf("standard product temporal workflow is not configured")
	}
	if err := client.StartStandardProduct(ctx, StandardProductWorkflowStartInput{
		TaskID:      strings.TrimSpace(taskID),
		RequestedAt: time.Now().UTC(),
	}); err != nil {
		return nil, err
	}
	return buildTaskGenerationActionTemporalResultPhase().run(
		assetGenerationActionRunStandardProductTemporal,
		req.ResponseMode,
		nil,
	), nil
}
