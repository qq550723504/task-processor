package listingkit

import (
	"context"
)

func (s *service) ExecuteTaskGenerationAction(ctx context.Context, taskID string, req *ExecuteGenerationActionRequest) (*GenerationActionExecutionResult, error) {
	return s.taskGenerationOrDefault().ExecuteTaskGenerationAction(ctx, taskID, req)
}

func resolveLayerTemporalPlatform(req *ExecuteGenerationActionRequest) string {
	return resolveTemporalRequestPlatform(req)
}
