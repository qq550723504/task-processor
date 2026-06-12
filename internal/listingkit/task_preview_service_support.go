package listingkit

import "context"

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

func (s *taskPreviewService) decorateTaskPreview(ctx context.Context, task *Task, preview *ListingKitPreview) {
	if s.decorateSheinStoreResolutionPreview != nil {
		s.decorateSheinStoreResolutionPreview(ctx, task, preview)
	}
}
