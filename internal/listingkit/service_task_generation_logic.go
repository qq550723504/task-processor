package listingkit

import "context"

func (s *service) GetTaskGenerationTasks(ctx context.Context, taskID string, query *GenerationTaskQuery) (*GenerationTaskPage, error) {
	return s.taskGenerationOrDefault().GetTaskGenerationTasks(ctx, taskID, query)
}

func (s *service) ExecuteTaskGenerationAction(ctx context.Context, taskID string, req *ExecuteGenerationActionRequest) (*GenerationActionExecutionResult, error) {
	return s.taskGenerationOrDefault().ExecuteTaskGenerationAction(ctx, taskID, req)
}

func (s *service) GetTaskGenerationQueue(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationQueuePage, error) {
	return s.taskGenerationOrDefault().GetTaskGenerationQueue(ctx, taskID, query)
}

func (s *service) GetTaskGenerationReviewPreview(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationReviewPreviewResponse, error) {
	return s.taskGenerationOrDefault().GetTaskGenerationReviewPreview(ctx, taskID, query)
}

func (s *service) GetTaskGenerationReviewSession(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationReviewSessionResponse, error) {
	return s.taskGenerationOrDefault().GetTaskGenerationReviewSession(ctx, taskID, query)
}

func (s *service) DispatchTaskGenerationNavigation(ctx context.Context, taskID string, req *GenerationReviewNavigationDispatchRequest) (*GenerationReviewNavigationDispatchResponse, error) {
	return s.taskGenerationOrDefault().DispatchTaskGenerationNavigation(ctx, taskID, req)
}

func (s *service) executeGenerationNavigationDispatchPlan(ctx context.Context, taskID string, target *GenerationReviewNavigationTarget, responseMode string) (*GenerationNavigationDispatchExecution, error) {
	return s.taskGenerationOrDefault().executeGenerationNavigationDispatchPlan(ctx, taskID, target, responseMode)
}

func (s *service) RetryTaskGenerationTasks(ctx context.Context, taskID string, req *RetryGenerationTasksRequest) (*GenerationTaskPage, error) {
	return s.taskGenerationOrDefault().RetryTaskGenerationTasks(ctx, taskID, req)
}
