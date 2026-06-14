package listingkit

import previewdomain "task-processor/internal/listing/preview"

func initializeListingKitPreview(task *Task, selectedPlatform string) (*ListingKitPreview, string, error) {
	if task == nil {
		return nil, "", ErrTaskNotFound
	}
	normalizedPlatform, ok := previewdomain.ValidateSelectedPlatform(selectedPlatform)
	if !ok {
		return nil, "", ErrUnsupportedPreviewPlatform
	}
	return buildBaseListingKitPreview(task, normalizedPlatform), normalizedPlatform, nil
}

func populatePendingListingKitPreview(task *Task, preview *ListingKitPreview) *ListingKitPreview {
	if preview == nil {
		return nil
	}
	preview.Overview = buildPendingPreviewHeader(task)
	return preview
}

func populateListingKitPreviewResult(task *Task, preview *ListingKitPreview, selectedPlatform string) error {
	if preview == nil || task == nil {
		return nil
	}
	ensureTaskPodExecution(task)
	attachListingKitPreviewResult(preview, task, selectedPlatform)
	return buildPreviewPlatformSections(task.Result, preview, selectedPlatform)
}
