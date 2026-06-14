package listingkit

import (
	"context"

	previewdomain "task-processor/internal/listing/preview"
)

type taskPreviewServiceConfig struct {
	repo       Repository
	decorators taskPreviewDecorationWiring
}

type taskPreviewReader interface {
	GetTaskPreview(context.Context, string, string) (*ListingKitPreview, error)
}

type taskPreviewService struct {
	reader *previewdomain.TaskPreviewService[Task, ListingKitPreview]
}

func newTaskPreviewService(config taskPreviewServiceConfig) *taskPreviewService {
	svc := &taskPreviewService{}
	svc.reader = previewdomain.NewTaskPreviewService(previewdomain.TaskPreviewServiceConfig[Task, ListingKitPreview]{
		Repository: config.repo,
		BuildPreview: func(ctx context.Context, task *Task, platform string) (*ListingKitPreview, error) {
			return buildTaskPreview(ctx, task, platform, config.decorators)
		},
		FinalizePreview: func(ctx context.Context, task *Task, preview *ListingKitPreview) error {
			return finalizeTaskPreview(ctx, task, preview, config.decorators)
		},
	})
	return svc
}

func (s *taskPreviewService) GetTaskPreview(ctx context.Context, taskID string, platform string) (*ListingKitPreview, error) {
	return s.reader.GetTaskPreview(ctx, taskID, platform)
}
