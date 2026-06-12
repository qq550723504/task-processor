package listingkit

import (
	"context"

	assetgeneration "task-processor/internal/asset/generation"
)

type taskExportServiceConfig struct {
	repo                     Repository
	listAssetGenerationTasks func(context.Context, string) ([]assetgeneration.Task, error)
}

type taskExportService struct {
	repo                     Repository
	listAssetGenerationTasks func(context.Context, string) ([]assetgeneration.Task, error)
}

func newTaskExportService(config taskExportServiceConfig) *taskExportService {
	return &taskExportService{
		repo:                     config.repo,
		listAssetGenerationTasks: config.listAssetGenerationTasks,
	}
}

func (s *taskExportService) GetTaskExport(ctx context.Context, taskID string, platform string) (*ListingKitExport, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	export, err := buildListingKitExport(task, platform)
	if err != nil {
		return nil, err
	}
	tasks, err := s.listAssetGenerationTasks(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	projection := buildAssetGenerationProjection(task.Result, tasks)
	export.AssetGenerationSummary = projection.Summary
	export.AssetGenerationTasks = projection.Tasks
	if len(export.AssetRenderPreviews) == 0 && task.Result != nil {
		export.AssetRenderPreviews = buildAssetRenderPreviews(task.Result.AssetBundle)
	}
	if len(export.PlatformAssetRenderPreviews) == 0 && task.Result != nil {
		export.PlatformAssetRenderPreviews = buildPlatformAssetRenderPreviews(task.Result)
	}
	export.AssetGenerationQueue = projection.Queue
	export.AssetGenerationOverview = projection.Overview
	return export, nil
}
