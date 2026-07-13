package httpapi

import "github.com/gin-gonic/gin"

type TaskActionRouteHandler interface {
	GenerateListingKit(c *gin.Context)
	ListTasks(c *gin.Context)
	GetSDSBaselineReadiness(c *gin.Context)
	WarmSDSBaseline(c *gin.Context)
	CreateSDSRetirementRun(c *gin.Context)
	GetSDSRetirementRun(c *gin.Context)
	UpdateSDSRetirementSelection(c *gin.Context)
	ConfirmSDSRetirementRun(c *gin.Context)
	RetrySDSRetirementRun(c *gin.Context)
	UploadListingKitImages(c *gin.Context)
	GetUploadedListingKitImage(c *gin.Context)
	DeleteUploadedListingKitImage(c *gin.Context)
	AnalyzeStudioReferenceStyle(c *gin.Context)
	GenerateStudioDesigns(c *gin.Context)
	GenerateStudioProductImages(c *gin.Context)
	StartStudioAsyncJob(c *gin.Context)
	GetStudioAsyncJob(c *gin.Context)
	RegenerateSheinDataImage(c *gin.Context)
	GetTaskResult(c *gin.Context)
	RequeuePendingTasks(c *gin.Context)
	RecoverTaskNow(c *gin.Context)
	BulkRecoverTasks(c *gin.Context)
	GetTaskPreview(c *gin.Context)
	GetTaskGenerationTasks(c *gin.Context)
	GetTaskGenerationQueue(c *gin.Context)
	GetTaskGenerationReviewSession(c *gin.Context)
	GetTaskGenerationReviewPreview(c *gin.Context)
	DispatchTaskGenerationNavigation(c *gin.Context)
	RetryTaskGenerationTasks(c *gin.Context)
	RetryTaskChildTask(c *gin.Context)
	GetTaskSDSRepair(c *gin.Context)
	RepairAndRetryTaskSDS(c *gin.Context)
	ExecuteTaskGenerationAction(c *gin.Context)
	GetTaskRevisionHistory(c *gin.Context)
	GetTaskRevisionHistoryDetail(c *gin.Context)
	GetTaskExport(c *gin.Context)
	ApplyTaskRevision(c *gin.Context)
	ValidateTaskRevision(c *gin.Context)
	SubmitTask(c *gin.Context)
	RefreshSubmissionStatus(c *gin.Context)
	PreviewSheinPrice(c *gin.Context)
	SearchSheinCategories(c *gin.Context)
	UpdateSheinFinalDraft(c *gin.Context)
	GetSubmissionEvents(c *gin.Context)
	ClearSheinResolutionCache(c *gin.Context)
}

type TaskRouteHandler interface {
	TaskActionRouteHandler
}

type StudioGenerationRouteHandler interface {
	AnalyzeStudioReferenceStyle(c *gin.Context)
	GenerateStudioDesigns(c *gin.Context)
	GenerateStudioProductImages(c *gin.Context)
	StartStudioAsyncJob(c *gin.Context)
	GetStudioAsyncJob(c *gin.Context)
}

type studioBatchRunRouteHandler interface {
	CreateStudioBatchRun(c *gin.Context)
	GetStudioBatchRun(c *gin.Context)
	ListStudioBatchRunItems(c *gin.Context)
	CancelStudioBatchRun(c *gin.Context)
	RecoverStudioBatchRun(c *gin.Context)
}
