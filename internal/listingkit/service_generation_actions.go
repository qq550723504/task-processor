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

func cloneGenerationQueueQuery(query *GenerationQueueQuery) *GenerationQueueQuery {
	if query == nil {
		return nil
	}
	cloned := *query
	return &cloned
}

func cloneRetryGenerationTasksRequest(req *RetryGenerationTasksRequest) *RetryGenerationTasksRequest {
	if req == nil {
		return nil
	}
	cloned := *req
	cloned.TaskIDs = append([]string(nil), req.TaskIDs...)
	cloned.Slots = append([]string(nil), req.Slots...)
	return &cloned
}
