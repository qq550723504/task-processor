package listingkit

import previewdomain "task-processor/internal/listing/preview"

func buildListingKitTaskPreviewDomainProjection(
	task *Task,
	readProjection *listingKitReadProjection,
	selectedPlatform string,
) *previewdomain.Preview {
	if task == nil || task.Result == nil || readProjection == nil {
		return nil
	}
	return previewdomain.BuildTaskReadModel(previewdomain.TaskReadModelInput{
		Task: previewdomain.TaskShellInput{
			TaskID:           task.ID,
			Status:           string(task.Status),
			SelectedPlatform: selectedPlatform,
			ResultPlatforms:  task.Result.Platforms,
			RequestPlatforms: previewRequestPlatforms(task),
			CreatedAt:        task.CreatedAt,
			UpdatedAt:        task.UpdatedAt,
		},
		ReadModel: readProjection.previewDomainReadModelInput(),
	})
}

func previewRequestPlatforms(task *Task) []string {
	if task == nil || task.Request == nil {
		return nil
	}
	return task.Request.Platforms
}
