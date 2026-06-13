package listingkit

type serviceCollaboratorMirrors struct {
	taskLifecycle             *taskLifecycleService
	taskGeneration            *taskGenerationService
	taskRevision              *taskRevisionService
	taskPreview               *taskPreviewService
	taskExport                *taskExportService
	sdsBaseline               *sdsBaselineService
	taskStudioSession         *taskStudioSessionService
	taskStudioBatchDraft      *taskStudioBatchDraftService
	studioBatchGeneration     *studioBatchGenerationService
	taskStudioBatch           *taskStudioBatchService
	studioBatchRunExecutor    *taskStudioBatchRunExecutor
	studioBatchRunCoordinator *studioBatchRunCoordinator
	taskStudioBatchRun        *taskStudioBatchRunService
	taskStudioMedia           *taskStudioMediaService
	settingsAdmin             *settingsAdminService
	sheinAdmin                *sheinAdminService
}
