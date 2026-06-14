package listingkit

import "context"

func (s *service) GetTaskPreview(ctx context.Context, taskID string, platform string) (*ListingKitPreview, error) {
	return s.taskPreviewOrDefault().GetTaskPreview(ctx, taskID, platform)
}

func (s *service) buildTaskPreview(ctx context.Context, task *Task, platform string) (*ListingKitPreview, error) {
	return buildTaskPreview(ctx, task, platform, buildTaskPreviewDecorationWiring(s))
}
