package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
	sheinpub "task-processor/internal/publishing/shein"
)

type handler struct {
	taskLifecycleService       listingkit.TaskLifecycleService
	taskRecoveryService        listingkit.TaskRecoveryService
	taskRequeueService         listingkit.TaskRequeueService
	sdsBaselineWarmService     sdsBaselineWarmService
	generationTaskService      listingkit.GenerationTaskService
	childTaskRetryService      childTaskRetryService
	studioMediaService         listingkit.StudioMediaService
	studioBatchRunService      studioBatchRunHandlerService
	studioSessionService       studioSessionAsyncJobService
	uploadedImageDeleteService uploadedImageDeleteService
	storeAdminService          storeAdminHandlerService
	storeRepository            listingadmin.StoreRepository
	sheinSyncService           listingkit.SheinSyncService
	sheinCandidateService      listingkit.SheinCandidateService
	sheinEnrollmentService     listingkit.SheinEnrollmentService
	sheinSyncRepository        listingkit.SheinSyncRepository
	studioAsyncJobs            *studioAsyncJobStore
	initErr                    error
	adminHandlers
	subscriptionDependencies
	settingsService settingsNamespaceService
}

type storeAdminHandlers struct {
	storeHandler           *listingadmin.StoreHandler
	storeStatisticsHandler *listingadmin.StoreStatisticsHandler
	importTaskHandler      *listingadmin.ImportTaskHandler
}

type catalogAdminHandlers struct {
	filterRuleHandler              *listingadmin.FilterRuleHandler
	profitRuleHandler              *listingadmin.ProfitRuleHandler
	pricingRuleHandler             *listingadmin.PricingRuleHandler
	operationStrategyHandler       *listingadmin.OperationStrategyHandler
	sensitiveWordHandler           *listingadmin.SensitiveWordHandler
	generationTopicCatalogHandler  *listingadmin.GenerationTopicCatalogHandler
	generationTopicPolicyHandler   *listingadmin.GenerationTopicPolicyHandler
	generationTopicOverrideHandler *listingadmin.GenerationTopicOverrideHandler
	productImportMappingHandler    *listingadmin.ProductImportMappingHandler
	categoryHandler                *listingadmin.CategoryHandler
	productDataHandler             *listingadmin.ProductDataHandler
}

type adminHandlers struct {
	storeAdminHandlers
	catalogAdminHandlers
}

type subscriptionDependencies struct {
	subscriptionService *listingsubscription.Service
	subscriptionHandler *listingsubscription.Handler
	platformAdminUsers  []string
	platformAdminRoles  []string
}

type handlerCoreService interface {
	listingkit.TaskLifecycleService
	listingkit.GenerationTaskService
	listingkit.StudioMediaService
}

type HandlerService interface {
	handlerCoreService
}

type storeAdminHandlerService interface {
	ListSheinStoreProfiles(ctx context.Context) ([]listingkit.ListingKitStoreProfile, error)
	UpsertSheinStoreProfile(ctx context.Context, req *listingkit.ListingKitStoreProfile) (*listingkit.ListingKitStoreProfile, error)
	DeleteSheinStoreProfile(ctx context.Context, id int64) error
	PreviewSheinPrice(ctx context.Context, taskID string, req *listingkit.SheinPricePreviewRequest) (*sheinpub.PricingReview, error)
	SearchSheinCategories(ctx context.Context, taskID string, query string) (*listingkit.SheinCategorySearchResult, error)
	UpdateSheinFinalDraft(ctx context.Context, taskID string, req *listingkit.SheinFinalDraftUpdateRequest) (*listingkit.ListingKitPreview, error)
	GetSubmissionEvents(ctx context.Context, taskID string) (*listingkit.SheinSubmissionEventPage, error)
	ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*listingkit.SheinResolutionCacheClearResult, error)
}

type childTaskRetryService interface {
	RetryTaskChildTask(ctx context.Context, taskID string, req *listingkit.RetryChildTaskRequest) (*listingkit.TaskResult, error)
}

type uploadedImageDeleteService interface {
	DeleteUploadedImage(ctx context.Context, key string) (*listingkit.DeletedUploadedImage, error)
}

type sdsBaselineWarmService interface {
	WarmSDSBaseline(ctx context.Context, req *listingkit.WarmSDSBaselineRequest) (*listingkit.SDSBaselineReadiness, error)
}

type studioSessionAsyncJobService interface {
	SyncStudioDesignAsyncJob(ctx context.Context, sessionID string, jobStatus listingkit.StudioAsyncJobStatus, jobID string, errMessage string) error
}

type studioBatchRunHandlerService interface {
	CreateStudioBatchRun(ctx context.Context, req *listingkit.CreateStudioBatchRunRequest) (*listingkit.StudioBatchRunRecord, []listingkit.StudioBatchRunItemRecord, error)
	GetStudioBatchRun(ctx context.Context, runID string) (*listingkit.StudioBatchRunRecord, error)
	ListStudioBatchRunItems(ctx context.Context, runID string) ([]listingkit.StudioBatchRunItemRecord, error)
	CancelStudioBatchRun(ctx context.Context, runID string) error
}

type HandlerOption func(*handler)

type AdminHandlerDependencies struct {
	StoreRepository                   listingadmin.StoreRepository
	StoreStatisticsRepository         listingadmin.StoreStatisticsRepository
	ImportTaskRepository              listingadmin.ImportTaskRepository
	FilterRuleRepository              listingadmin.FilterRuleRepository
	ProfitRuleRepository              listingadmin.ProfitRuleRepository
	PricingRuleRepository             listingadmin.PricingRuleRepository
	OperationStrategyRepository       listingadmin.OperationStrategyRepository
	SensitiveWordRepository           listingadmin.SensitiveWordRepository
	GenerationTopicOverrideRepository listingadmin.GenerationTopicOverrideRepository
	GenerationTopicPolicyRepository   listingadmin.GenerationTopicPolicyRepository
	ProductImportMappingRepository    listingadmin.ProductImportMappingRepository
	CategoryRepository                listingadmin.CategoryRepository
	ProductDataRepository             listingadmin.ProductDataRepository
}

type SubscriptionDependencies struct {
	Service            *listingsubscription.Service
	PlatformAdminUsers []string
	PlatformAdminRoles []string
}

type HandlerDependencies struct {
	StudioAsyncJobRepository listingkit.StudioAsyncJobRepository
	Admin                    AdminHandlerDependencies
	Subscription             SubscriptionDependencies
}

func withHandlerState(apply func(*handler)) HandlerOption {
	return func(h *handler) {
		if h == nil || apply == nil {
			return
		}
		apply(h)
	}
}

func withAdminHandlers(apply func(*adminHandlers)) HandlerOption {
	return withHandlerState(func(h *handler) {
		apply(&h.adminHandlers)
	})
}

func withAdminDependency[Repo comparable](repo Repo, apply func(Repo, *adminHandlers)) HandlerOption {
	return withAdminHandlers(func(admin *adminHandlers) {
		var zero Repo
		if repo == zero {
			return
		}
		apply(repo, admin)
	})
}

func WithDependencies(deps HandlerDependencies) HandlerOption {
	options := []HandlerOption{
		WithStudioAsyncJobRepository(deps.StudioAsyncJobRepository),
		withStoreAdminDependencies(deps.Admin),
		withCatalogAdminDependencies(deps.Admin),
		withSubscriptionConfig(deps.Subscription),
	}
	return func(h *handler) {
		for _, option := range options {
			if option != nil {
				option(h)
			}
		}
	}
}

func WithStudioAsyncJobRepository(repo listingkit.StudioAsyncJobRepository) HandlerOption {
	return withHandlerState(func(h *handler) {
		store, err := newStudioAsyncJobStore(repo)
		if err != nil {
			h.initErr = err
			return
		}
		h.studioAsyncJobs = store
	})
}

func WithStudioBatchRunService(service studioBatchRunHandlerService) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.studioBatchRunService = service
	})
}

func WithTaskLifecycleService(service listingkit.TaskLifecycleService) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.taskLifecycleService = service
	})
}

func WithTaskRecoveryService(service listingkit.TaskRecoveryService) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.taskRecoveryService = service
	})
}

func WithTaskRequeueService(service listingkit.TaskRequeueService) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.taskRequeueService = service
	})
}

func WithGenerationTaskService(service listingkit.GenerationTaskService) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.generationTaskService = service
	})
}

func WithStudioMediaService(service listingkit.StudioMediaService) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.studioMediaService = service
	})
}

func WithStudioSessionAsyncJobService(service studioSessionAsyncJobService) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.studioSessionService = service
	})
}

func WithUploadedImageDeleteService(service uploadedImageDeleteService) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.uploadedImageDeleteService = service
	})
}

func WithChildTaskRetryService(service childTaskRetryService) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.childTaskRetryService = service
	})
}

func WithStoreAdminService(service storeAdminHandlerService) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.storeAdminService = service
	})
}

func WithSettingsHandlerService(service settingsHandlerService) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.attachSettingsService(service)
	})
}

func WithSheinSyncServices(
	syncService listingkit.SheinSyncService,
	candidateService listingkit.SheinCandidateService,
	enrollmentService listingkit.SheinEnrollmentService,
) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.sheinSyncService = syncService
		h.sheinCandidateService = candidateService
		h.sheinEnrollmentService = enrollmentService
	})
}

func WithSheinSyncRepository(repo listingkit.SheinSyncRepository) HandlerOption {
	return withHandlerState(func(h *handler) {
		h.sheinSyncRepository = repo
	})
}

func newHandlerWithDefaults(studioAsyncJobs *studioAsyncJobStore) *handler {
	return &handler{
		studioBatchRunService: nil,
		studioAsyncJobs:       studioAsyncJobs,
	}
}

func (h *handler) attachCoreServices(service handlerCoreService) {
	if service == nil {
		return
	}
	h.taskLifecycleService = service
	h.generationTaskService = service
	h.studioMediaService = service
}

func (h *handler) attachOptionalServices(service any) {
	if service == nil {
		return
	}
	if adminService, ok := any(service).(storeAdminHandlerService); ok {
		h.storeAdminService = adminService
	}
	if settingsService, ok := any(service).(settingsHandlerService); ok && h.settingsService == nil {
		h.attachSettingsService(settingsService)
	}
	if retryService, ok := service.(childTaskRetryService); ok {
		h.childTaskRetryService = retryService
	}
	if recoveryService, ok := any(service).(listingkit.TaskRecoveryService); ok {
		h.taskRecoveryService = recoveryService
	}
	if requeueService, ok := any(service).(listingkit.TaskRequeueService); ok {
		h.taskRequeueService = requeueService
	}
	if sessionService, ok := service.(studioSessionAsyncJobService); ok {
		h.studioSessionService = sessionService
	}
	if batchRunService, ok := service.(studioBatchRunHandlerService); ok {
		h.studioBatchRunService = batchRunService
	}
	if warmService, ok := service.(sdsBaselineWarmService); ok {
		h.sdsBaselineWarmService = warmService
	}
	if deleteService, ok := service.(uploadedImageDeleteService); ok {
		h.uploadedImageDeleteService = deleteService
	}
}

func (h *handler) attachSettingsService(service settingsHandlerService) {
	if service == nil {
		return
	}
	h.settingsService = newSettingsService(service)
}

func (h *handler) applyOptions(opts []HandlerOption) {
	for _, opt := range opts {
		if opt != nil {
			opt(h)
		}
	}
}

func (h *handler) finalize() error {
	if h.initErr != nil {
		return h.initErr
	}
	if h.taskLifecycleService == nil {
		return errors.New("task lifecycle service is not configured")
	}
	if h.generationTaskService == nil {
		return errors.New("generation task service is not configured")
	}
	if h.studioMediaService == nil {
		return errors.New("studio media service is not configured")
	}
	return nil
}

func NewHandler(service HandlerService, opts ...HandlerOption) (*handler, error) {
	studioAsyncJobs, err := newStudioAsyncJobStore(nil)
	if err != nil {
		return nil, err
	}
	h := newHandlerWithDefaults(studioAsyncJobs)
	h.attachCoreServices(service)
	h.attachOptionalServices(service)
	h.applyOptions(opts)
	if err := h.finalize(); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *handler) GenerateListingKit(c *gin.Context) {
	if !h.requireSubscription(c, listingsubscription.ModuleStudio) {
		return
	}
	var req listingkit.GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	req.ImageURLs = absolutizeUploadedImageURLs(c, req.ImageURLs)
	req.TenantID = requestTenantID(c, req.TenantID)
	if req.UserID == "" {
		req.UserID = requestUserID(c)
	}
	task, err := h.taskLifecycleService.CreateGenerateTask(requestContext(c, req.TenantID), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "invalid request") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "task_creation_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"task_id": task.ID, "tenant_id": task.TenantID, "status": task.Status, "created_at": task.CreatedAt})
}

func (h *handler) ListTasks(c *gin.Context) {
	var query listingkit.TaskListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	query.TenantID = requestTenantID(c, query.TenantID)
	page, err := h.taskLifecycleService.ListTasks(requestContext(c, query.TenantID), &query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "task_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *handler) GetTaskResult(c *gin.Context) {
	result, err := h.taskLifecycleService.GetTaskResult(requestContext(c), c.Param("task_id"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, listingkit.ErrTaskNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "query_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
