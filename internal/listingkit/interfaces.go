package listingkit

import (
	"context"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/catalog/canonical"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
	sheinpub "task-processor/internal/publishing/shein"
)

type TaskSubmitter interface{ Submit(taskID string) error }

type ProductService interface {
	CreateGenerateTask(ctx context.Context, req *productenrich.GenerateRequest) (*productenrich.Task, error)
	GetTaskResult(ctx context.Context, taskID string) (*productenrich.TaskResult, error)
	ProcessProduct(ctx context.Context, task *productenrich.Task) (*productenrich.ProductJSON, error)
}

type ImageService interface {
	CreateProcessTask(ctx context.Context, req *productimage.ImageProcessRequest) (*productimage.Task, error)
	GetTaskResult(ctx context.Context, taskID string) (*productimage.TaskResult, error)
	ProcessImages(ctx context.Context, task *productimage.Task) (*productimage.ImageProcessResult, error)
}

type AIClientCredentialStore interface {
	SaveCredential(ctx context.Context, credential openaiclient.AIClientCredential) error
	GetCredential(ctx context.Context, tenantID, userID, clientName string) (*openaiclient.AIClientCredential, error)
}

type Repository interface {
	CreateTask(ctx context.Context, task *Task) error
	GetTask(ctx context.Context, taskID string) (*Task, error)
	ListTasks(ctx context.Context, query *TaskListQuery) ([]Task, int64, error)
	MarkProcessing(ctx context.Context, taskID string) error
	MarkCompleted(ctx context.Context, taskID string, result *ListingKitResult) error
	MarkNeedsReview(ctx context.Context, taskID string, result *ListingKitResult, reason string) error
	MarkFailed(ctx context.Context, taskID string, errorMsg string) error
	PrepareRetry(ctx context.Context, taskID string) error
	IncrementRetryCount(ctx context.Context, taskID string) error
	SaveTaskResult(ctx context.Context, taskID string, result *ListingKitResult) error
}

type TaskListSummarySource interface {
	ListTaskSummaryTasks(ctx context.Context, query *TaskListQuery) ([]Task, error)
}

type CanonicalProductCacheRepository interface {
	GetCanonicalProductCache(ctx context.Context, fingerprint string) (*canonical.Product, error)
	SaveCanonicalProductCache(ctx context.Context, fingerprint string, product *canonical.Product, sourceTaskID string) error
}

type SDSBaselineCacheRepository interface {
	// tenantID is optional. When empty, implementations resolve the tenant from ctx.
	// If both tenantID and ctx resolve to a tenant, they must match or the call fails.
	GetSDSBaselineCache(ctx context.Context, tenantID string, baselineKey string) (*SDSBaselineCacheEntry, error)
	// entry.TenantID follows the same contract as GetSDSBaselineCache's tenantID argument.
	SaveSDSBaselineCache(ctx context.Context, entry *SDSBaselineCacheEntry) error
}

type Assembler interface {
	Assemble(task *Task, canonical *canonical.Product, image *productimage.ImageProcessResult) *ListingKitResult
}

type AmazonDraftBuilder interface {
	Build(req *GenerateRequest, canonical *canonical.Product, image *productimage.ImageProcessResult) *amazonlisting.AmazonListingDraft
}

type TaskLifecycleService interface {
	CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error)
	ListTasks(ctx context.Context, query *TaskListQuery) (*TaskListPage, error)
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
	GenerateStudioDesigns(ctx context.Context, req *StudioDesignRequest) (*StudioDesignResponse, error)
	GenerateStudioProductImages(ctx context.Context, req *StudioProductImageRequest) (*StudioProductImageResponse, error)
	RegenerateSheinDataImage(ctx context.Context, taskID string, req *RegenerateSheinDataImageRequest) (*RegenerateSheinDataImageResponse, error)
}

type StoreAdminService interface {
	ListSheinStoreProfiles(ctx context.Context) ([]ListingKitStoreProfile, error)
	UpsertSheinStoreProfile(ctx context.Context, req *ListingKitStoreProfile) (*ListingKitStoreProfile, error)
	DeleteSheinStoreProfile(ctx context.Context, id int64) error
	GetSheinStoreRoutingSettings(ctx context.Context) (*ListingKitStoreRoutingSettings, error)
	UpdateSheinStoreRoutingSettings(ctx context.Context, req *ListingKitStoreRoutingSettings) (*ListingKitStoreRoutingSettings, error)
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

type TaskSubmitterConfigurer interface {
	SetTaskSubmitter(submitter TaskSubmitter)
}

type InternalListingKitService interface {
	ProcessListingKit(ctx context.Context, task *Task) (*ListingKitResult, error)
	ProcessStandardProductLayer(ctx context.Context, taskID string) (*StandardProductSnapshot, error)
	ProcessPlatformAdaptationLayer(ctx context.Context, taskID string, platform string) (*ListingKitResult, error)
}

type WorkflowClientConfigurer interface {
	ConfigureSheinPublishWorkflowClient(client SheinPublishWorkflowClient, enabled bool)
	ConfigureStandardProductWorkflowClient(client StandardProductWorkflowClient, enabled bool)
	ConfigurePlatformAdaptWorkflowClient(client PlatformAdaptWorkflowClient, enabled bool)
}

type Service interface {
	TaskLifecycleService
	GenerationTaskService
	StudioMediaService
	StoreAdminService
	InternalListingKitService
}
