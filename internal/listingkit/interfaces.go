package listingkit

import (
	"context"

	"github.com/gin-gonic/gin"

	"task-processor/internal/amazonlisting"
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/listingkit/reviewstore"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
	sheinpub "task-processor/internal/publishing/shein"
	sdsusecase "task-processor/internal/sds/usecase"
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

type SDSSyncService = sdsusecase.Service

type AssetRepository = assetrepo.Repository
type AssetGenerationService = assetgeneration.Service
type AssetRecipeResolver = assetrecipe.Resolver
type AssetBundleBuilder = assetbundle.Builder
type GenerationReviewRepository = reviewstore.Repository
type SheinManagementClient = management.ClientManager

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

type CanonicalProductCacheRepository interface {
	GetCanonicalProductCache(ctx context.Context, fingerprint string) (*productenrich.CanonicalProduct, error)
	SaveCanonicalProductCache(ctx context.Context, fingerprint string, product *productenrich.CanonicalProduct, sourceTaskID string) error
}

type Assembler interface {
	Assemble(task *Task, canonical *productenrich.CanonicalProduct, image *productimage.ImageProcessResult) *ListingKitResult
}

type AmazonDraftBuilder interface {
	Build(req *GenerateRequest, canonical *productenrich.CanonicalProduct, image *productimage.ImageProcessResult) *amazonlisting.AmazonListingDraft
}

type Service interface {
	CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error)
	ListTasks(ctx context.Context, query *TaskListQuery) (*TaskListPage, error)
	UploadImages(ctx context.Context, req *UploadImagesRequest) (*UploadImagesResponse, error)
	GetUploadedImage(ctx context.Context, key string) (*UploadedImageFile, error)
	GenerateStudioDesigns(ctx context.Context, req *StudioDesignRequest) (*StudioDesignResponse, error)
	GenerateStudioProductImages(ctx context.Context, req *StudioProductImageRequest) (*StudioProductImageResponse, error)
	RegenerateSheinDataImage(ctx context.Context, taskID string, req *RegenerateSheinDataImageRequest) (*RegenerateSheinDataImageResponse, error)
	GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error)
	GetTaskPreview(ctx context.Context, taskID string, platform string) (*ListingKitPreview, error)
	GetTaskGenerationTasks(ctx context.Context, taskID string, query *GenerationTaskQuery) (*GenerationTaskPage, error)
	GetTaskGenerationQueue(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationQueuePage, error)
	GetTaskGenerationReviewSession(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationReviewSessionResponse, error)
	GetTaskGenerationReviewPreview(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationReviewPreviewResponse, error)
	DispatchTaskGenerationNavigation(ctx context.Context, taskID string, req *GenerationReviewNavigationDispatchRequest) (*GenerationReviewNavigationDispatchResponse, error)
	RetryTaskGenerationTasks(ctx context.Context, taskID string, req *RetryGenerationTasksRequest) (*GenerationTaskPage, error)
	ExecuteTaskGenerationAction(ctx context.Context, taskID string, req *ExecuteGenerationActionRequest) (*GenerationActionExecutionResult, error)
	GetTaskRevisionHistory(ctx context.Context, taskID string, query *RevisionHistoryQuery) (*ListingKitRevisionHistoryPage, error)
	GetTaskRevisionHistoryDetail(ctx context.Context, taskID string, revisionID string, query *RevisionHistoryDetailQuery) (*ListingKitRevisionHistoryDetail, error)
	GetTaskExport(ctx context.Context, taskID string, platform string) (*ListingKitExport, error)
	ApplyTaskRevision(ctx context.Context, taskID string, req *ApplyRevisionRequest) (*ListingKitPreview, error)
	ValidateTaskRevision(ctx context.Context, taskID string, req *ApplyRevisionRequest) (*RevisionValidationResult, error)
	SubmitTask(ctx context.Context, taskID string, req *SubmitTaskRequest) (*ListingKitPreview, error)
	RefreshSubmissionStatus(ctx context.Context, taskID string) (*ListingKitPreview, error)
	GetSheinSettings(ctx context.Context) (*SheinSettings, error)
	UpdateSheinSettings(ctx context.Context, req *SheinSettings) (*SheinSettings, error)
	PreviewSheinPrice(ctx context.Context, taskID string, req *SheinPricePreviewRequest) (*sheinpub.PricingReview, error)
	SearchSheinCategories(ctx context.Context, taskID string, query string) (*SheinCategorySearchResult, error)
	UpdateSheinFinalDraft(ctx context.Context, taskID string, req *SheinFinalDraftUpdateRequest) (*ListingKitPreview, error)
	GetSubmissionEvents(ctx context.Context, taskID string) (*SheinSubmissionEventPage, error)
	ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*SheinResolutionCacheClearResult, error)
	ProcessListingKit(ctx context.Context, task *Task) (*ListingKitResult, error)
	SetTaskSubmitter(submitter TaskSubmitter)
}

type HandlerService interface {
	CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error)
	ListTasks(ctx context.Context, query *TaskListQuery) (*TaskListPage, error)
	UploadImages(ctx context.Context, req *UploadImagesRequest) (*UploadImagesResponse, error)
	GetUploadedImage(ctx context.Context, key string) (*UploadedImageFile, error)
	GenerateStudioDesigns(ctx context.Context, req *StudioDesignRequest) (*StudioDesignResponse, error)
	GenerateStudioProductImages(ctx context.Context, req *StudioProductImageRequest) (*StudioProductImageResponse, error)
	RegenerateSheinDataImage(ctx context.Context, taskID string, req *RegenerateSheinDataImageRequest) (*RegenerateSheinDataImageResponse, error)
	GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error)
	GetTaskPreview(ctx context.Context, taskID string, platform string) (*ListingKitPreview, error)
	GetTaskGenerationTasks(ctx context.Context, taskID string, query *GenerationTaskQuery) (*GenerationTaskPage, error)
	GetTaskGenerationQueue(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationQueuePage, error)
	GetTaskGenerationReviewSession(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationReviewSessionResponse, error)
	GetTaskGenerationReviewPreview(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationReviewPreviewResponse, error)
	DispatchTaskGenerationNavigation(ctx context.Context, taskID string, req *GenerationReviewNavigationDispatchRequest) (*GenerationReviewNavigationDispatchResponse, error)
	RetryTaskGenerationTasks(ctx context.Context, taskID string, req *RetryGenerationTasksRequest) (*GenerationTaskPage, error)
	ExecuteTaskGenerationAction(ctx context.Context, taskID string, req *ExecuteGenerationActionRequest) (*GenerationActionExecutionResult, error)
	GetTaskRevisionHistory(ctx context.Context, taskID string, query *RevisionHistoryQuery) (*ListingKitRevisionHistoryPage, error)
	GetTaskRevisionHistoryDetail(ctx context.Context, taskID string, revisionID string, query *RevisionHistoryDetailQuery) (*ListingKitRevisionHistoryDetail, error)
	GetTaskExport(ctx context.Context, taskID string, platform string) (*ListingKitExport, error)
	ApplyTaskRevision(ctx context.Context, taskID string, req *ApplyRevisionRequest) (*ListingKitPreview, error)
	ValidateTaskRevision(ctx context.Context, taskID string, req *ApplyRevisionRequest) (*RevisionValidationResult, error)
	SubmitTask(ctx context.Context, taskID string, req *SubmitTaskRequest) (*ListingKitPreview, error)
	RefreshSubmissionStatus(ctx context.Context, taskID string) (*ListingKitPreview, error)
	GetSheinSettings(ctx context.Context) (*SheinSettings, error)
	UpdateSheinSettings(ctx context.Context, req *SheinSettings) (*SheinSettings, error)
	PreviewSheinPrice(ctx context.Context, taskID string, req *SheinPricePreviewRequest) (*sheinpub.PricingReview, error)
	SearchSheinCategories(ctx context.Context, taskID string, query string) (*SheinCategorySearchResult, error)
	UpdateSheinFinalDraft(ctx context.Context, taskID string, req *SheinFinalDraftUpdateRequest) (*ListingKitPreview, error)
	GetSubmissionEvents(ctx context.Context, taskID string) (*SheinSubmissionEventPage, error)
	ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*SheinResolutionCacheClearResult, error)
}

type Handler interface {
	GenerateListingKit(c *gin.Context)
	ListTasks(c *gin.Context)
	UploadListingKitImages(c *gin.Context)
	GetUploadedListingKitImage(c *gin.Context)
	GenerateStudioDesigns(c *gin.Context)
	GenerateStudioProductImages(c *gin.Context)
	RegenerateSheinDataImage(c *gin.Context)
	GetTaskResult(c *gin.Context)
	GetTaskPreview(c *gin.Context)
	GetTaskGenerationTasks(c *gin.Context)
	GetTaskGenerationQueue(c *gin.Context)
	GetTaskGenerationReviewSession(c *gin.Context)
	GetTaskGenerationReviewPreview(c *gin.Context)
	DispatchTaskGenerationNavigation(c *gin.Context)
	RetryTaskGenerationTasks(c *gin.Context)
	ExecuteTaskGenerationAction(c *gin.Context)
	GetTaskRevisionHistory(c *gin.Context)
	GetTaskRevisionHistoryDetail(c *gin.Context)
	GetTaskExport(c *gin.Context)
	ApplyTaskRevision(c *gin.Context)
	ValidateTaskRevision(c *gin.Context)
	SubmitTask(c *gin.Context)
	RefreshSubmissionStatus(c *gin.Context)
	GetSheinSettings(c *gin.Context)
	UpdateSheinSettings(c *gin.Context)
	PreviewSheinPrice(c *gin.Context)
	SearchSheinCategories(c *gin.Context)
	UpdateSheinFinalDraft(c *gin.Context)
	GetSubmissionEvents(c *gin.Context)
	ClearSheinResolutionCache(c *gin.Context)
}
