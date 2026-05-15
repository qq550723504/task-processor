package listingkit

import (
	"context"

	"github.com/gin-gonic/gin"

	"task-processor/internal/amazonlisting"
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/catalog/canonical"
	"task-processor/internal/infra/clients/management"
	openaiclient "task-processor/internal/infra/clients/openai"
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

type Assembler interface {
	Assemble(task *Task, canonical *canonical.Product, image *productimage.ImageProcessResult) *ListingKitResult
}

type AmazonDraftBuilder interface {
	Build(req *GenerateRequest, canonical *canonical.Product, image *productimage.ImageProcessResult) *amazonlisting.AmazonListingDraft
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
	GetAIClientSettings(ctx context.Context, scope string, clientName string) (*AIClientSettings, error)
	UpdateAIClientSettings(ctx context.Context, req *AIClientSettings) (*AIClientSettings, error)
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
	GetAIClientSettings(ctx context.Context, scope string, clientName string) (*AIClientSettings, error)
	UpdateAIClientSettings(ctx context.Context, req *AIClientSettings) (*AIClientSettings, error)
	PreviewSheinPrice(ctx context.Context, taskID string, req *SheinPricePreviewRequest) (*sheinpub.PricingReview, error)
	SearchSheinCategories(ctx context.Context, taskID string, query string) (*SheinCategorySearchResult, error)
	UpdateSheinFinalDraft(ctx context.Context, taskID string, req *SheinFinalDraftUpdateRequest) (*ListingKitPreview, error)
	GetSubmissionEvents(ctx context.Context, taskID string) (*SheinSubmissionEventPage, error)
	ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*SheinResolutionCacheClearResult, error)
}

type Handler interface {
	GenerateListingKit(c *gin.Context)
	ListTasks(c *gin.Context)
	ListAdminStores(c *gin.Context)
	GetAdminStore(c *gin.Context)
	CreateAdminStore(c *gin.Context)
	UpdateAdminStore(c *gin.Context)
	UpdateAdminStoreStatus(c *gin.Context)
	DeleteAdminStore(c *gin.Context)
	ListSimpleAdminStores(c *gin.Context)
	ListAdminStoreStatistics(c *gin.Context)
	ListDeletedAdminStores(c *gin.Context)
	RestoreAdminStore(c *gin.Context)
	PermanentlyDeleteAdminStore(c *gin.Context)
	ExtendAdminStoreValidity(c *gin.Context)
	ListAdminImportTasks(c *gin.Context)
	BatchCreateAdminImportTasks(c *gin.Context)
	DeleteAdminImportTask(c *gin.Context)
	ListAdminFilterRules(c *gin.Context)
	GetAdminFilterRule(c *gin.Context)
	CreateAdminFilterRule(c *gin.Context)
	UpdateAdminFilterRule(c *gin.Context)
	UpdateAdminFilterRuleStatus(c *gin.Context)
	DeleteAdminFilterRule(c *gin.Context)
	ListAdminProfitRules(c *gin.Context)
	GetAdminProfitRule(c *gin.Context)
	CreateAdminProfitRule(c *gin.Context)
	UpdateAdminProfitRule(c *gin.Context)
	UpdateAdminProfitRuleStatus(c *gin.Context)
	DeleteAdminProfitRule(c *gin.Context)
	ListAdminPricingRules(c *gin.Context)
	GetAdminPricingRule(c *gin.Context)
	CreateAdminPricingRule(c *gin.Context)
	UpdateAdminPricingRule(c *gin.Context)
	UpdateAdminPricingRuleStatus(c *gin.Context)
	DeleteAdminPricingRule(c *gin.Context)
	ListAdminOperationStrategies(c *gin.Context)
	GetAdminOperationStrategy(c *gin.Context)
	CreateAdminOperationStrategy(c *gin.Context)
	UpdateAdminOperationStrategy(c *gin.Context)
	UpdateAdminOperationStrategyStatus(c *gin.Context)
	DeleteAdminOperationStrategy(c *gin.Context)
	ListAdminSensitiveWords(c *gin.Context)
	GetAdminSensitiveWord(c *gin.Context)
	CreateAdminSensitiveWord(c *gin.Context)
	UpdateAdminSensitiveWord(c *gin.Context)
	UpdateAdminSensitiveWordStatus(c *gin.Context)
	DeleteAdminSensitiveWord(c *gin.Context)
	ListAdminProductImportMappings(c *gin.Context)
	GetAdminProductImportMapping(c *gin.Context)
	CreateAdminProductImportMapping(c *gin.Context)
	UpdateAdminProductImportMapping(c *gin.Context)
	UpdateAdminProductImportMappingStatus(c *gin.Context)
	DeleteAdminProductImportMapping(c *gin.Context)
	ListAdminCategories(c *gin.Context)
	GetAdminCategory(c *gin.Context)
	CreateAdminCategory(c *gin.Context)
	UpdateAdminCategory(c *gin.Context)
	UpdateAdminCategoryStatus(c *gin.Context)
	DeleteAdminCategory(c *gin.Context)
	ListAdminProductData(c *gin.Context)
	GetAdminProductData(c *gin.Context)
	CreateAdminProductData(c *gin.Context)
	UpdateAdminProductData(c *gin.Context)
	UpdateAdminProductDataStatus(c *gin.Context)
	DeleteAdminProductData(c *gin.Context)
	GetCurrentSubscription(c *gin.Context)
	ListSubscriptionModules(c *gin.Context)
	ListSubscriptionEntitlements(c *gin.Context)
	UpsertSubscriptionEntitlement(c *gin.Context)
	GetPlatformTenantSubscription(c *gin.Context)
	UpsertPlatformTenantSubscriptionEntitlement(c *gin.Context)
	UploadListingKitImages(c *gin.Context)
	GetUploadedListingKitImage(c *gin.Context)
	GenerateStudioDesigns(c *gin.Context)
	GenerateStudioProductImages(c *gin.Context)
	StartStudioAsyncJob(c *gin.Context)
	GetStudioAsyncJob(c *gin.Context)
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
	GetAIClientSettings(c *gin.Context)
	UpdateAIClientSettings(c *gin.Context)
	PreviewSheinPrice(c *gin.Context)
	SearchSheinCategories(c *gin.Context)
	UpdateSheinFinalDraft(c *gin.Context)
	GetSubmissionEvents(c *gin.Context)
	ClearSheinResolutionCache(c *gin.Context)
}
