package listingkit

import (
	"context"

	assetgeneration "task-processor/internal/asset/generation"
)

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
	export.AssetGenerationSummary = buildAssetGenerationSummary(tasks)
	export.AssetGenerationTasks = append([]assetgeneration.Task(nil), tasks...)
	if len(export.AssetRenderPreviews) == 0 && task.Result != nil {
		export.AssetRenderPreviews = buildAssetRenderPreviews(task.Result.AssetBundle)
	}
	if len(export.PlatformAssetRenderPreviews) == 0 && task.Result != nil {
		export.PlatformAssetRenderPreviews = buildPlatformAssetRenderPreviews(task.Result)
	}
	export.AssetGenerationQueue = buildGenerationWorkQueue(withListingKitResultGeneration(task.Result, tasks))
	export.AssetGenerationOverview = buildAssetGenerationOverview(export.AssetGenerationQueue)
	return export, nil
}
