package listingkit

import "strings"

func previewPlatforms(task *Task) []string {
	if task == nil {
		return nil
	}
	if task.Result != nil && len(task.Result.Platforms) > 0 {
		return append([]string(nil), task.Result.Platforms...)
	}
	if task.Request != nil && len(task.Request.Platforms) > 0 {
		return append([]string(nil), task.Request.Platforms...)
	}
	return nil
}

func buildListingKitPreview(task *Task, selectedPlatform string) (*ListingKitPreview, error) {
	if task == nil {
		return nil, ErrTaskNotFound
	}
	selectedPlatform = normalizePreviewPlatform(selectedPlatform)
	if selectedPlatform != "" && len(normalizePlatforms([]string{selectedPlatform})) == 0 {
		return nil, ErrUnsupportedPreviewPlatform
	}

	preview := buildBaseListingKitPreview(task, selectedPlatform)
	if task.Result == nil {
		preview.Overview = &ListingKitPreviewHeader{
			StatusMessage: previewStatusMessage(task.Status),
		}
		return preview, nil
	}
	ensureTaskPodExecution(task)

	attachListingKitPreviewResult(preview, task.Result, selectedPlatform)

	for _, builder := range previewPlatformBuilders() {
		if err := builder.build(task, preview, selectedPlatform); err != nil {
			return nil, err
		}
	}

	return preview, nil
}

func buildBaseListingKitPreview(task *Task, selectedPlatform string) *ListingKitPreview {
	preview := &ListingKitPreview{
		TaskID:           task.ID,
		Status:           task.Status,
		SelectedPlatform: selectedPlatform,
		Platforms:        previewPlatforms(task),
		CreatedAt:        task.CreatedAt,
	}
	if task.Status == TaskStatusCompleted || task.Status == TaskStatusNeedsReview || task.Status == TaskStatusFailed {
		completedAt := task.UpdatedAt
		preview.CompletedAt = &completedAt
	}
	return preview
}

func attachListingKitPreviewResult(preview *ListingKitPreview, result *ListingKitResult, selectedPlatform string) {
	preview.Overview = buildPreviewHeader(result, selectedPlatform)
	preview.NeedsReview = result.Summary != nil && result.Summary.NeedsReview
	preview.Catalog = result.CatalogProduct
	preview.Assets = result.AssetBundle
	preview.AssetInventory = result.AssetInventorySummary
	preview.AssetRenderPreviews = append([]AssetRenderPreview(nil), result.AssetRenderPreviews...)
	preview.PlatformAssetRenderPreviews = append([]PlatformAssetRenderPreviews(nil), result.PlatformAssetRenderPreviews...)
	if len(preview.AssetRenderPreviews) == 0 {
		preview.AssetRenderPreviews = buildAssetRenderPreviews(result.AssetBundle)
	}
	if len(preview.PlatformAssetRenderPreviews) == 0 {
		preview.PlatformAssetRenderPreviews = buildPlatformAssetRenderPreviews(result)
	}
	preview.PlatformAssetRenderPreviews = filterPlatformAssetRenderPreviews(preview.PlatformAssetRenderPreviews, selectedPlatform)
	preview.AssetGenerationQueue = result.AssetGenerationQueue
	preview.AssetGenerationOverview = result.AssetGenerationOverview
	preview.RevisionHistoryMeta = buildRevisionHistoryMeta(result)
	preview.RevisionHistory = buildRevisionHistoryPreviewItems(result.RevisionHistory)
}

func previewStatusFromReviewNotes(reviewNotes []string) string {
	if len(reviewNotes) > 0 {
		return "needs_review"
	}
	return "ready"
}

func previewStatusMessage(status TaskStatus) string {
	switch status {
	case TaskStatusPending:
		return "任务已创建，预览结果尚未生成"
	case TaskStatusProcessing:
		return "任务处理中，预览结果尚未准备完成"
	case TaskStatusNeedsReview:
		return "任务已完成，等待人工审核"
	case TaskStatusFailed:
		return "任务执行失败，暂无预览结果"
	default:
		return ""
	}
}

func normalizePreviewPlatform(platform string) string {
	return strings.ToLower(strings.TrimSpace(platform))
}

func shouldBuildPreviewPlatform(selectedPlatform, platform string) bool {
	return selectedPlatform == "" || isSelectedPreviewPlatform(selectedPlatform, platform)
}

func isSelectedPreviewPlatform(selectedPlatform, platform string) bool {
	return selectedPlatform == platform
}

func buildPreviewHeader(result *ListingKitResult, selectedPlatform string) *ListingKitPreviewHeader {
	if result == nil {
		return nil
	}

	header := &ListingKitPreviewHeader{
		Country:       result.Country,
		Language:      result.Language,
		StatusMessage: "预览结果已生成",
	}
	if result.Summary != nil {
		header.SourceType = result.Summary.SourceType
		header.ImageCount = result.Summary.ImageCount
		header.VariantCount = result.Summary.VariantCount
		header.Warnings = append([]string(nil), result.Summary.Warnings...)
	}
	header.ReviewReasons = reviewReasonsFromResult(result)
	header.PlatformCards = buildPlatformPreviewCards(result, selectedPlatform)
	return header
}
