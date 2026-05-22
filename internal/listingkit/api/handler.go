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
)

type handler struct {
	taskLifecycleService       listingkit.TaskLifecycleService
	generationTaskService      listingkit.GenerationTaskService
	childTaskRetryService      childTaskRetryService
	studioMediaService         listingkit.StudioMediaService
	uploadedImageDeleteService uploadedImageDeleteService
	storeAdminService          listingkit.StoreAdminService
	studioAsyncJobs            *studioAsyncJobStore
	initErr                    error
	adminHandlers
	subscriptionDependencies
	settingsService *settingsService
}

type storeAdminHandlers struct {
	storeHandler           *listingadmin.StoreHandler
	storeStatisticsHandler *listingadmin.StoreStatisticsHandler
	importTaskHandler      *listingadmin.ImportTaskHandler
}

type catalogAdminHandlers struct {
	filterRuleHandler           *listingadmin.FilterRuleHandler
	profitRuleHandler           *listingadmin.ProfitRuleHandler
	pricingRuleHandler          *listingadmin.PricingRuleHandler
	operationStrategyHandler    *listingadmin.OperationStrategyHandler
	sensitiveWordHandler        *listingadmin.SensitiveWordHandler
	productImportMappingHandler *listingadmin.ProductImportMappingHandler
	categoryHandler             *listingadmin.CategoryHandler
	productDataHandler          *listingadmin.ProductDataHandler
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

type HandlerService interface {
	listingkit.TaskLifecycleService
	listingkit.GenerationTaskService
	listingkit.StudioMediaService
	listingkit.StoreAdminService
}

type childTaskRetryService interface {
	RetryTaskChildTask(ctx context.Context, taskID string, req *listingkit.RetryChildTaskRequest) (*listingkit.TaskResult, error)
}

type uploadedImageDeleteService interface {
	DeleteUploadedImage(ctx context.Context, key string) (*listingkit.DeletedUploadedImage, error)
}

type HandlerOption func(*handler)

type AdminHandlerDependencies struct {
	StoreRepository                listingadmin.StoreRepository
	StoreStatisticsRepository      listingadmin.StoreStatisticsRepository
	ImportTaskRepository           listingadmin.ImportTaskRepository
	FilterRuleRepository           listingadmin.FilterRuleRepository
	ProfitRuleRepository           listingadmin.ProfitRuleRepository
	PricingRuleRepository          listingadmin.PricingRuleRepository
	OperationStrategyRepository    listingadmin.OperationStrategyRepository
	SensitiveWordRepository        listingadmin.SensitiveWordRepository
	ProductImportMappingRepository listingadmin.ProductImportMappingRepository
	CategoryRepository             listingadmin.CategoryRepository
	ProductDataRepository          listingadmin.ProductDataRepository
}

type SubscriptionDependencies struct {
	Service            *listingsubscription.Service
	PlatformAdminUsers []string
	PlatformAdminRoles []string
}

type HandlerDependencies struct {
	StudioAsyncJobStorePath string
	Admin                   AdminHandlerDependencies
	Subscription            SubscriptionDependencies
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

func withSubscriptionDependencies(apply func(*subscriptionDependencies)) HandlerOption {
	return withHandlerState(func(h *handler) {
		apply(&h.subscriptionDependencies)
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

func withSubscriptionDependency[Dep comparable](dep Dep, apply func(Dep, *subscriptionDependencies)) HandlerOption {
	return withSubscriptionDependencies(func(subscription *subscriptionDependencies) {
		var zero Dep
		if dep == zero {
			return
		}
		apply(dep, subscription)
	})
}

func WithDependencies(deps HandlerDependencies) HandlerOption {
	options := []HandlerOption{
		WithStudioAsyncJobStorePath(deps.StudioAsyncJobStorePath),
		WithPlatformSubscriptionAccess(deps.Subscription.PlatformAdminUsers, deps.Subscription.PlatformAdminRoles),
		WithStoreRepository(deps.Admin.StoreRepository),
		WithStoreStatisticsRepository(deps.Admin.StoreStatisticsRepository),
		WithImportTaskRepository(deps.Admin.ImportTaskRepository),
		WithFilterRuleRepository(deps.Admin.FilterRuleRepository),
		WithProfitRuleRepository(deps.Admin.ProfitRuleRepository),
		WithPricingRuleRepository(deps.Admin.PricingRuleRepository),
		WithOperationStrategyRepository(deps.Admin.OperationStrategyRepository),
		WithSensitiveWordRepository(deps.Admin.SensitiveWordRepository),
		WithProductImportMappingRepository(deps.Admin.ProductImportMappingRepository),
		WithCategoryRepository(deps.Admin.CategoryRepository),
		WithProductDataRepository(deps.Admin.ProductDataRepository),
		WithSubscriptionService(deps.Subscription.Service),
	}
	return func(h *handler) {
		for _, option := range options {
			if option != nil {
				option(h)
			}
		}
	}
}

func WithStoreRepository(repo listingadmin.StoreRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.StoreRepository, admin *adminHandlers) {
		admin.storeHandler = listingadmin.NewStoreHandler(repo)
	})
}

func WithStoreStatisticsRepository(repo listingadmin.StoreStatisticsRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.StoreStatisticsRepository, admin *adminHandlers) {
		admin.storeStatisticsHandler = listingadmin.NewStoreStatisticsHandler(repo)
	})
}

func WithImportTaskRepository(repo listingadmin.ImportTaskRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.ImportTaskRepository, admin *adminHandlers) {
		admin.importTaskHandler = listingadmin.NewImportTaskHandler(repo)
	})
}

func WithFilterRuleRepository(repo listingadmin.FilterRuleRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.FilterRuleRepository, admin *adminHandlers) {
		admin.filterRuleHandler = listingadmin.NewFilterRuleHandler(repo)
	})
}

func WithProfitRuleRepository(repo listingadmin.ProfitRuleRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.ProfitRuleRepository, admin *adminHandlers) {
		admin.profitRuleHandler = listingadmin.NewProfitRuleHandler(repo)
	})
}

func WithPricingRuleRepository(repo listingadmin.PricingRuleRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.PricingRuleRepository, admin *adminHandlers) {
		admin.pricingRuleHandler = listingadmin.NewPricingRuleHandler(repo)
	})
}

func WithOperationStrategyRepository(repo listingadmin.OperationStrategyRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.OperationStrategyRepository, admin *adminHandlers) {
		admin.operationStrategyHandler = listingadmin.NewOperationStrategyHandler(repo)
	})
}

func WithSensitiveWordRepository(repo listingadmin.SensitiveWordRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.SensitiveWordRepository, admin *adminHandlers) {
		admin.sensitiveWordHandler = listingadmin.NewSensitiveWordHandler(repo)
	})
}

func WithProductImportMappingRepository(repo listingadmin.ProductImportMappingRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.ProductImportMappingRepository, admin *adminHandlers) {
		admin.productImportMappingHandler = listingadmin.NewProductImportMappingHandler(repo)
	})
}

func WithCategoryRepository(repo listingadmin.CategoryRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.CategoryRepository, admin *adminHandlers) {
		admin.categoryHandler = listingadmin.NewCategoryHandler(repo)
	})
}

func WithProductDataRepository(repo listingadmin.ProductDataRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.ProductDataRepository, admin *adminHandlers) {
		admin.productDataHandler = listingadmin.NewProductDataHandler(repo)
	})
}

func WithSubscriptionService(service *listingsubscription.Service) HandlerOption {
	return withSubscriptionDependency(service, func(service *listingsubscription.Service, subscription *subscriptionDependencies) {
		subscription.subscriptionService = service
		subscription.subscriptionHandler = listingsubscription.NewHandler(service)
	})
}

func WithPlatformSubscriptionAccess(users []string, roles []string) HandlerOption {
	return withSubscriptionDependencies(func(subscription *subscriptionDependencies) {
		subscription.platformAdminUsers = append([]string(nil), users...)
		subscription.platformAdminRoles = append([]string(nil), roles...)
	})
}

func WithStudioAsyncJobStorePath(path string) HandlerOption {
	return withHandlerState(func(h *handler) {
		store, err := newStudioAsyncJobStoreWithPath(path)
		if err != nil {
			h.initErr = err
			return
		}
		h.studioAsyncJobs = store
	})
}

func newHandlerWithDefaults(service HandlerService, studioAsyncJobs *studioAsyncJobStore) *handler {
	return &handler{
		taskLifecycleService:  service,
		generationTaskService: service,
		studioMediaService:    service,
		storeAdminService:     service,
		studioAsyncJobs:       studioAsyncJobs,
	}
}

func (h *handler) attachOptionalServices(service HandlerService) {
	if retryService, ok := service.(childTaskRetryService); ok {
		h.childTaskRetryService = retryService
	}
	if deleteService, ok := service.(uploadedImageDeleteService); ok {
		h.uploadedImageDeleteService = deleteService
	}
}

func (h *handler) applyOptions(opts []HandlerOption) {
	for _, opt := range opts {
		if opt != nil {
			opt(h)
		}
	}
}

func (h *handler) finalize() error {
	h.settingsService = newSettingsService(h.storeAdminService)
	if h.initErr != nil {
		return h.initErr
	}
	return nil
}

func NewHandler(service HandlerService, opts ...HandlerOption) (*handler, error) {
	if service == nil {
		return nil, errors.New("service cannot be nil")
	}
	studioAsyncJobs, err := newDefaultStudioAsyncJobStore()
	if err != nil {
		return nil, err
	}
	h := newHandlerWithDefaults(service, studioAsyncJobs)
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
