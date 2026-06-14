package listingkit

import previewdomain "task-processor/internal/listing/preview"

func buildBaseListingKitPreview(task *Task, selectedPlatform string) *ListingKitPreview {
	var resultPlatforms []string
	if task != nil && task.Result != nil {
		resultPlatforms = task.Result.Platforms
	}
	var requestPlatforms []string
	if task != nil && task.Request != nil {
		requestPlatforms = task.Request.Platforms
	}
	base := previewdomain.BuildTaskShell(previewdomain.TaskShellInput{
		TaskID:           task.ID,
		Status:           string(task.Status),
		SelectedPlatform: selectedPlatform,
		ResultPlatforms:  resultPlatforms,
		RequestPlatforms: requestPlatforms,
		CreatedAt:        task.CreatedAt,
		UpdatedAt:        task.UpdatedAt,
	})
	return adaptPreviewDomainShell(base)
}
