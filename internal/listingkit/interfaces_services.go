package listingkit

import (
	"context"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

type TaskLifecycleService interface {
	CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error)
	ListTasks(ctx context.Context, query *TaskListQuery) (*TaskListPage, error)
	GetSDSBaselineReadiness(ctx context.Context, query *SDSBaselineReadinessQuery) (*SDSBaselineReadiness, error)
	GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error)
	GetTaskPreview(ctx context.Context, taskID string, platform string) (*ListingKitPreview, error)
	GetTaskRevisionHistory(ctx context.Context, taskID string, query *RevisionHistoryQuery) (*ListingKitRevisionHistoryPage, error)
	GetTaskRevisionHistoryDetail(ctx context.Context, taskID string, revisionID string, query *RevisionHistoryDetailQuery) (*ListingKitRevisionHistoryDetail, error)
	GetTaskExport(ctx context.Context, taskID string, platform string) (*ListingKitExport, error)
	ApplyTaskRevision(ctx context.Context, taskID string, req *ApplyRevisionRequest) (*ListingKitPreview, error)
	ValidateTaskRevision(ctx context.Context, taskID string, req *ApplyRevisionRequest) (*RevisionValidationResult, error)
	SubmitTask(ctx context.Context, taskID string, req *SubmitTaskRequest) (*ListingKitPreview, error)
	RefreshSubmissionStatus(ctx context.Context, taskID string) (*ListingKitPreview, error)
}

type TaskRecoveryService interface {
	// RecoverTaskNow is the authoritative high-level recovery path. It validates
	// task recoverability, performs repository state transitions, re-submits the
	// task, and restores durable blocked/failed state if submission recovery
	// persistence encounters errors. This path is intended for explicit user
	// action and forces immediate recovery for blocked_retryable tasks without
	// waiting for next_retry_at.
	RecoverTaskNow(ctx context.Context, taskID string) (*Task, error)
	RunRecoverySweep(ctx context.Context, now time.Time, limit int) (int64, error)
	BulkRecoverTasks(ctx context.Context, query *RecoverBlockedTasksQuery) (int64, error)
}

type SDSChildRetrySweepService interface {
	RunDueSDSChildRetries(ctx context.Context, now time.Time, limit int) (int64, error)
}

type TaskRequeueService interface {
	RequeuePendingTasks(ctx context.Context, req *RequeuePendingTasksRequest) (*RequeuePendingTasksResult, error)
}

type SDSBaselineWarmService interface {
	WarmSDSBaseline(ctx context.Context, req *WarmSDSBaselineRequest) (*SDSBaselineReadiness, error)
}

type SDSRetirementService interface {
	CreateSDSRetirementRun(ctx context.Context, req *CreateSDSRetirementRunRequest) (*SDSRetirementRunDetail, error)
	GetSDSRetirementRun(ctx context.Context, runID string) (*SDSRetirementRunDetail, error)
	UpdateSDSRetirementSelection(ctx context.Context, runID string, req *UpdateSDSRetirementSelectionRequest) (*SDSRetirementRunDetail, error)
	ConfirmSDSRetirementRun(ctx context.Context, runID string, req *ConfirmSDSRetirementRunRequest) (*SDSRetirementRunDetail, error)
	RetrySDSRetirementRun(ctx context.Context, runID string) (*SDSRetirementRunDetail, error)
}

type GenerationTaskService interface {
	GetTaskGenerationTasks(ctx context.Context, taskID string, query *GenerationTaskQuery) (*GenerationTaskPage, error)
	GetTaskGenerationQueue(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationQueuePage, error)
	GetTaskGenerationReviewSession(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationReviewSessionResponse, error)
	GetTaskGenerationReviewPreview(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationReviewPreviewResponse, error)
	DispatchTaskGenerationNavigation(ctx context.Context, taskID string, req *GenerationReviewNavigationDispatchRequest) (*GenerationReviewNavigationDispatchResponse, error)
	RetryTaskGenerationTasks(ctx context.Context, taskID string, req *RetryGenerationTasksRequest) (*GenerationTaskPage, error)
	ExecuteTaskGenerationAction(ctx context.Context, taskID string, req *ExecuteGenerationActionRequest) (*GenerationActionExecutionResult, error)
}

type StudioMediaService interface {
	UploadImages(ctx context.Context, req *UploadImagesRequest) (*UploadImagesResponse, error)
	GetUploadedImage(ctx context.Context, key string) (*UploadedImageFile, error)
	AnalyzeStudioReferenceStyle(ctx context.Context, req *StudioReferenceAnalysisRequest) (*StudioReferenceAnalysisResponse, error)
	GenerateStudioDesigns(ctx context.Context, req *StudioDesignRequest) (*StudioDesignResponse, error)
	GenerateStudioProductImages(ctx context.Context, req *StudioProductImageRequest) (*StudioProductImageResponse, error)
	RegenerateSheinDataImage(ctx context.Context, taskID string, req *RegenerateSheinDataImageRequest) (*RegenerateSheinDataImageResponse, error)
}

type StoreAdminService interface {
	ListSheinStoreProfiles(ctx context.Context) ([]ListingKitStoreProfile, error)
	UpsertSheinStoreProfile(ctx context.Context, req *ListingKitStoreProfile) (*ListingKitStoreProfile, error)
	DeleteSheinStoreProfile(ctx context.Context, id int64) error
	GetSheinSettings(ctx context.Context) (*SheinSettings, error)
	UpdateSheinSettings(ctx context.Context, req *SheinSettings) (*SheinSettings, error)
	GetAIClientSettings(ctx context.Context, scope string, clientName string) (*AIClientSettings, error)
	UpdateAIClientSettings(ctx context.Context, req *AIClientSettings) (*AIClientSettings, error)
	PreviewSheinPrice(ctx context.Context, taskID string, req *SheinPricePreviewRequest) (*sheinpub.PricingReview, error)
	SearchSheinCategories(ctx context.Context, taskID string, query string) (*SheinCategorySearchResult, error)
	UpdateSheinFinalDraft(ctx context.Context, taskID string, req *SheinFinalDraftUpdateRequest) (*ListingKitPreview, error)
	GetSubmissionEvents(ctx context.Context, taskID string) (*SheinSubmissionEventPage, error)
	ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*SheinResolutionCacheClearResult, error)
}

type InternalListingKitService interface {
	ProcessListingKit(ctx context.Context, task *Task) (*ListingKitResult, error)
	ProcessStandardProductLayer(ctx context.Context, taskID string) (*StandardProductSnapshot, error)
	ProcessPlatformAdaptationLayer(ctx context.Context, taskID string, platform string) (*ListingKitResult, error)
}
