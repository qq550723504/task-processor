package listingkit

import (
	"context"

	assetgeneration "task-processor/internal/asset/generation"
)

type taskPreviewServiceConfig struct {
	repo                                   Repository
	listAssetGenerationTasks               func(context.Context, string) ([]assetgeneration.Task, error)
	decorateSheinCookieAvailabilityPreview func(context.Context, *Task, *ListingKitPreview)
	decorateSheinStoreResolutionPreview    func(context.Context, *Task, *ListingKitPreview)
}

type taskPreviewService struct {
	repo                                   Repository
	listAssetGenerationTasks               func(context.Context, string) ([]assetgeneration.Task, error)
	decorateSheinCookieAvailabilityPreview func(context.Context, *Task, *ListingKitPreview)
	decorateSheinStoreResolutionPreview    func(context.Context, *Task, *ListingKitPreview)
}

func newTaskPreviewService(config taskPreviewServiceConfig) *taskPreviewService {
	return &taskPreviewService{
		repo:                                   config.repo,
		listAssetGenerationTasks:               config.listAssetGenerationTasks,
		decorateSheinCookieAvailabilityPreview: config.decorateSheinCookieAvailabilityPreview,
		decorateSheinStoreResolutionPreview:    config.decorateSheinStoreResolutionPreview,
	}
}

func (s *taskPreviewService) GetTaskPreview(ctx context.Context, taskID string, platform string) (*ListingKitPreview, error) {
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
	preview.AssetGenerationQueue = projection.Queue
	preview.AssetGenerationOverview = projection.Overview
	backfillTaskPreviewRenderPreviews(preview, task.Result)
	s.decorateTaskPreview(ctx, task, preview)
	return preview, nil
}

func (s *taskPreviewService) buildTaskPreview(ctx context.Context, task *Task, platform string) (*ListingKitPreview, error) {
	preview, err := buildListingKitPreview(task, platform)
	if err != nil {
		return nil, err
	}
	if s.decorateSheinCookieAvailabilityPreview != nil {
		s.decorateSheinCookieAvailabilityPreview(ctx, task, preview)
	}
	return preview, nil
}
