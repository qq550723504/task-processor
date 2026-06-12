package listingkit

import previewdomain "task-processor/internal/listing/preview"

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
		StatusMessage: previewdomain.StatusMessage(string(task.Status)),
	}
}

func previewStatusFromReviewNotes(reviewNotes []string) string {
	return previewdomain.StatusFromReviewReasons(reviewNotes)
}

func previewStatusMessage(status TaskStatus) string {
	return previewdomain.StatusMessage(string(status))
}
