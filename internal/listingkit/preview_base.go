package listingkit

import (
	"time"

	previewdomain "task-processor/internal/listing/preview"
)

func buildBaseListingKitPreview(task *Task, selectedPlatform string) *ListingKitPreview {
	var completedAt *time.Time
	if task.Status == TaskStatusCompleted || task.Status == TaskStatusNeedsReview || task.Status == TaskStatusFailed {
		value := task.UpdatedAt
		completedAt = &value
	}
	base := previewdomain.BuildProjection(previewdomain.ProjectionInput{
		Shell: previewdomain.ShellInput{
			TaskID:           task.ID,
			Status:           string(task.Status),
			SelectedPlatform: selectedPlatform,
			Platforms:        previewPlatforms(task),
			CreatedAt:        task.CreatedAt,
			CompletedAt:      completedAt,
		},
	})
	return adaptPreviewDomainShell(base)
}
