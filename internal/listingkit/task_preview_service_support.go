package listingkit

import (
	"context"

	assetgeneration "task-processor/internal/asset/generation"
)

type taskPreviewDecorationWiring struct {
	listAssetGenerationTasks               func(context.Context, string) ([]assetgeneration.Task, error)
	decorateSheinCookieAvailabilityPreview func(context.Context, *Task, *ListingKitPreview)
	decorateSheinStoreResolutionPreview    func(context.Context, *Task, *ListingKitPreview)
}

type taskPreviewAccessWiring struct {
	getTaskPreview   func(context.Context, string, string) (*ListingKitPreview, error)
	buildTaskPreview func(context.Context, *Task, string) (*ListingKitPreview, error)
}

func buildTaskPreviewAccessWiring(s *service) taskPreviewAccessWiring {
	return taskPreviewAccessWiring{
		getTaskPreview: func(ctx context.Context, taskID string, platform string) (*ListingKitPreview, error) {
			return s.GetTaskPreview(ctx, taskID, platform)
		},
		buildTaskPreview: func(ctx context.Context, task *Task, platform string) (*ListingKitPreview, error) {
			return s.buildTaskPreview(ctx, task, platform)
		},
	}
}

func buildTaskPreviewDecorationWiring(s *service) taskPreviewDecorationWiring {
	return taskPreviewDecorationWiring{
		listAssetGenerationTasks: func(ctx context.Context, taskID string) ([]assetgeneration.Task, error) {
			return s.listAssetGenerationTasks(ctx, taskID)
		},
		decorateSheinCookieAvailabilityPreview: func(ctx context.Context, task *Task, preview *ListingKitPreview) {
			s.decorateSheinCookieAvailabilityPreview(ctx, task, preview)
		},
		decorateSheinStoreResolutionPreview: func(ctx context.Context, task *Task, preview *ListingKitPreview) {
			s.decorateSheinStoreResolutionPreview(ctx, task, preview)
		},
	}
}

func buildTaskPreview(ctx context.Context, task *Task, platform string, decorators taskPreviewDecorationWiring) (*ListingKitPreview, error) {
	preview, err := buildListingKitPreview(task, platform)
	if err != nil {
		return nil, err
	}
	if decorators.decorateSheinCookieAvailabilityPreview != nil {
		decorators.decorateSheinCookieAvailabilityPreview(ctx, task, preview)
	}
	return preview, nil
}

func finalizeTaskPreview(ctx context.Context, task *Task, preview *ListingKitPreview, decorators taskPreviewDecorationWiring) error {
	tasks, err := decorators.listAssetGenerationTasks(ctx, task.ID)
	if err != nil {
		return err
	}
	projection := buildAssetGenerationProjection(task.Result, tasks)
	applyAssetGenerationProjectionToPreview(preview, projection)
	backfillTaskPreviewRenderPreviews(preview, task.Result)
	if decorators.decorateSheinStoreResolutionPreview != nil {
		decorators.decorateSheinStoreResolutionPreview(ctx, task, preview)
	}
	return nil
}

func backfillTaskPreviewRenderPreviews(preview *ListingKitPreview, result *ListingKitResult) {
	if preview == nil || result == nil {
		return
	}
	if len(preview.AssetRenderPreviews) == 0 {
		preview.AssetRenderPreviews = buildAssetRenderPreviews(result.AssetBundle)
	}
	if len(preview.PlatformAssetRenderPreviews) == 0 {
		preview.PlatformAssetRenderPreviews = buildPlatformAssetRenderPreviews(result)
	}
}
