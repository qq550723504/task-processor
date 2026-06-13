package listingkit

func initializeListingKitPreview(task *Task, selectedPlatform string) (*ListingKitPreview, string, error) {
	if task == nil {
		return nil, "", ErrTaskNotFound
	}
	normalizedPlatform, err := validateSelectedPreviewPlatform(selectedPlatform)
	if err != nil {
		return nil, "", err
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
	attachListingKitPreviewResult(preview, task.Result, selectedPlatform)
	return buildPreviewPlatformSections(task.Result, preview, selectedPlatform)
}
