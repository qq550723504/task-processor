package listingkit

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
	selectedPlatform, err := validateSelectedPreviewPlatform(selectedPlatform)
	if err != nil {
		return nil, err
	}

	preview := buildBaseListingKitPreview(task, selectedPlatform)
	if shouldBuildPendingPreview(task) {
		preview.Overview = buildPendingPreviewHeader(task)
		return preview, nil
	}

	ensureTaskPodExecution(task)
	attachListingKitPreviewResult(preview, task.Result, selectedPlatform)

	if err := buildPreviewPlatformSections(task, preview, selectedPlatform); err != nil {
		return nil, err
	}

	return preview, nil
}

func shouldBuildPendingPreview(task *Task) bool {
	return task == nil || task.Result == nil
}

func buildPendingPreviewHeader(task *Task) *ListingKitPreviewHeader {
	if task == nil {
		return nil
	}
	return &ListingKitPreviewHeader{
		StatusMessage: previewStatusMessage(task.Status),
	}
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
