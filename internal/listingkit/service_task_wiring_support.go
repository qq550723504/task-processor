package listingkit

import (
	"context"

	assetgeneration "task-processor/internal/asset/generation"
)

type taskPreviewExportReadWiring struct {
	repo                     Repository
	listAssetGenerationTasks func(context.Context, string) ([]assetgeneration.Task, error)
}

func buildTaskPreviewExportReadWiring(s *service) taskPreviewExportReadWiring {
	return taskPreviewExportReadWiring{
		repo: s.repo,
		listAssetGenerationTasks: func(ctx context.Context, taskID string) ([]assetgeneration.Task, error) {
			return s.listAssetGenerationTasks(ctx, taskID)
		},
	}
}
