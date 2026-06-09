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
