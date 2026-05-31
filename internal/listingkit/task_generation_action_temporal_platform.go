package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type taskGenerationActionTemporalPlatformPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationActionTemporalPlatformPhase(service *taskGenerationService) *taskGenerationActionTemporalPlatformPhase {
	return &taskGenerationActionTemporalPlatformPhase{service: service}
}

func (p *taskGenerationActionTemporalPlatformPhase) run(
	ctx context.Context,
	taskID string,
	req *ExecuteGenerationActionRequest,
) (*GenerationActionExecutionResult, error) {
	client, enabled := p.service.platformAdaptWorkflow()
	if !enabled || client == nil {
		return nil, fmt.Errorf("platform adaptation temporal workflow is not configured")
	}

	platform := resolveLayerTemporalPlatform(req)
	if err := client.StartPlatformAdaptation(ctx, PlatformAdaptWorkflowStartInput{
		TaskID:      strings.TrimSpace(taskID),
		Platform:    platform,
		RequestedAt: time.Now().UTC(),
	}); err != nil {
		return nil, err
	}

	return buildTaskGenerationActionTemporalResultPhase().run(
		assetGenerationActionRunPlatformAdaptTemporal,
		req.ResponseMode,
		&GenerationQueueQuery{Platform: platform},
	), nil
}
