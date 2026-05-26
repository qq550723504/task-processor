package listingkit

import "context"

func (s *service) GetTaskGenerationReviewSession(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationReviewSessionResponse, error) {
	return s.taskGenerationOrDefault().GetTaskGenerationReviewSession(ctx, taskID, query)
}
