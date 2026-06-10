package listingkit

import "context"

func (s *service) GetTaskExport(ctx context.Context, taskID string, platform string) (*ListingKitExport, error) {
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
