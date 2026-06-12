package listingkit

type studioCollaborators struct {
	session         *taskStudioSessionService
	batchDraft      *taskStudioBatchDraftService
	media           *taskStudioMediaService
	batchGeneration *studioBatchGenerationService
	batch           *taskStudioBatchService
	batchRun        *taskStudioBatchRunService
	runExecutor     *taskStudioBatchRunExecutor
	runCoordinator  *studioBatchRunCoordinator
}
