package listingkit

import (
	"context"

	assetgeneration "task-processor/internal/asset/generation"
)

func (s *service) GetTaskPreview(ctx context.Context, taskID string, platform string) (*ListingKitPreview, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	preview, err := buildListingKitPreview(task, platform)
	if err != nil {
		return nil, err
	}
	tasks, err := s.listAssetGenerationTasks(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	preview.AssetGenerationSummary = buildAssetGenerationSummary(tasks)
	preview.AssetGenerationTasks = append([]assetgeneration.Task(nil), tasks...)
	if len(preview.AssetRenderPreviews) == 0 && task.Result != nil {
		preview.AssetRenderPreviews = buildAssetRenderPreviews(task.Result.AssetBundle)
	}
	if len(preview.PlatformAssetRenderPreviews) == 0 && task.Result != nil {
		preview.PlatformAssetRenderPreviews = buildPlatformAssetRenderPreviews(task.Result)
	}
	preview.AssetGenerationQueue = buildGenerationWorkQueue(withListingKitResultGeneration(task.Result, tasks))
	preview.AssetGenerationOverview = buildAssetGenerationOverview(preview.AssetGenerationQueue)
	return preview, nil
}
