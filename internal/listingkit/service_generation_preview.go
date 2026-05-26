package listingkit

import "context"

func (s *service) GetTaskGenerationReviewPreview(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationReviewPreviewResponse, error) {
	return s.taskGenerationOrDefault().GetTaskGenerationReviewPreview(ctx, taskID, query)
}

func (in *GenerationReviewToolbarInput) GetViewer() *GenerationReviewPreviewViewer {
	if in == nil {
		return nil
	}
	return in.PreviewViewer
}
