package listingkit

import previewdomain "task-processor/internal/listing/preview"

func previewPlatforms(task *Task) []string {
	if task == nil {
		return nil
	}
	var resultPlatforms []string
	if task.Result != nil {
		resultPlatforms = task.Result.Platforms
	}
	var requestPlatforms []string
	if task.Request != nil {
		requestPlatforms = task.Request.Platforms
	}
	return previewdomain.ResolvePlatforms(resultPlatforms, requestPlatforms)
}

func buildListingKitPreview(task *Task, selectedPlatform string) (*ListingKitPreview, error) {
	preview, selectedPlatform, err := initializeListingKitPreview(task, selectedPlatform)
	if err != nil {
		return nil, err
	}

	if shouldBuildPendingPreview(task) {
		return populatePendingListingKitPreview(task, preview), nil
	}

	return preview, populateListingKitPreviewResult(task, preview, selectedPlatform)
}

func shouldBuildPendingPreview(task *Task) bool {
	return task == nil || task.Result == nil
}

func buildPendingPreviewHeader(task *Task) *ListingKitPreviewHeader {
	if task == nil {
		return nil
	}
	return adaptPreviewDomainHeader(previewdomain.BuildHeader(previewdomain.HeaderInput{
		StatusMessage: previewdomain.StatusMessage(string(task.Status)),
	}))
}

func previewStatusFromReviewNotes(reviewNotes []string) string {
	return previewdomain.StatusFromReviewReasons(reviewNotes)
}

func previewStatusMessage(status TaskStatus) string {
	return previewdomain.StatusMessage(string(status))
}
