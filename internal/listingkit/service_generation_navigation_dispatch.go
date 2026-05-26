package listingkit

import (
	"context"
)

func (s *service) DispatchTaskGenerationNavigation(ctx context.Context, taskID string, req *GenerationReviewNavigationDispatchRequest) (*GenerationReviewNavigationDispatchResponse, error) {
	return s.taskGenerationOrDefault().DispatchTaskGenerationNavigation(ctx, taskID, req)
}

func (s *service) executeGenerationNavigationDispatchPlan(ctx context.Context, taskID string, target *GenerationReviewNavigationTarget, responseMode string) (*GenerationNavigationDispatchExecution, error) {
	return s.taskGenerationOrDefault().executeGenerationNavigationDispatchPlan(ctx, taskID, target, responseMode)
}
