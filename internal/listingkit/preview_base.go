package listingkit

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
