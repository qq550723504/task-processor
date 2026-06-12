package listingkit

type taskCollaborators struct {
	lifecycle   *taskLifecycleService
	generation  *taskGenerationService
	revision    *taskRevisionService
	preview     *taskPreviewService
	export      *taskExportService
	sdsBaseline *sdsBaselineService
}
