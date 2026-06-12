package listingkit

import "context"

func (s *service) GetTaskPreview(ctx context.Context, taskID string, platform string) (*ListingKitPreview, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	preview, err := s.buildTaskPreview(ctx, task, platform)
	if err != nil {
		return nil, err
	}
	tasks, err := s.listAssetGenerationTasks(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	projection := buildAssetGenerationProjection(task.Result, tasks)
	preview.AssetGenerationSummary = projection.Summary
	preview.AssetGenerationTasks = projection.Tasks
	if len(preview.AssetRenderPreviews) == 0 && task.Result != nil {
		preview.AssetRenderPreviews = buildAssetRenderPreviews(task.Result.AssetBundle)
	}
	if len(preview.PlatformAssetRenderPreviews) == 0 && task.Result != nil {
		preview.PlatformAssetRenderPreviews = buildPlatformAssetRenderPreviews(task.Result)
	}
	preview.AssetGenerationQueue = projection.Queue
	preview.AssetGenerationOverview = projection.Overview
	s.decorateSheinStoreResolutionPreview(ctx, task, preview)
	return preview, nil
}

func (s *service) buildTaskPreview(ctx context.Context, task *Task, platform string) (*ListingKitPreview, error) {
	preview, err := buildListingKitPreview(task, platform)
	if err != nil {
		return nil, err
	}
	s.decorateSheinCookieAvailabilityPreview(ctx, task, preview)
	return preview, nil
}
