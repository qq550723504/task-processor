package listingkit

type taskCollaborators struct {
	lifecycle   *taskLifecycleService
	generation  *taskGenerationService
	revision    *taskRevisionService
	preview     taskPreviewReader
	export      *taskExportService
	sdsBaseline *sdsBaselineService
}
