package listingkit

type taskCollaborators struct {
	lifecycle   *taskLifecycleService
	revision    *taskRevisionService
	preview     taskPreviewReader
	export      *taskExportService
	sdsBaseline *sdsBaselineService
}
