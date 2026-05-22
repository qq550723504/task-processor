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
	taskLifecycleService        listingkit.TaskLifecycleService
	generationTaskService       listingkit.GenerationTaskService
	childTaskRetryService       childTaskRetryService
	studioMediaService          listingkit.StudioMediaService
	uploadedImageDeleteService  uploadedImageDeleteService
	storeAdminService           listingkit.StoreAdminService
	studioAsyncJobs             *studioAsyncJobStore
	initErr                     error
	adminHandlers
	subscriptionDependencies
	settingsService *settingsService
}

type adminHandlers struct {
	storeHandler                *listingadmin.StoreHandler
	storeStatisticsHandler      *listingadmin.StoreStatisticsHandler
	importTaskHandler           *listingadmin.ImportTaskHandler
	filterRuleHandler           *listingadmin.FilterRuleHandler
	profitRuleHandler           *listingadmin.ProfitRuleHandler
	pricingRuleHandler          *listingadmin.PricingRuleHandler
	operationStrategyHandler    *listingadmin.OperationStrategyHandler
	sensitiveWordHandler        *listingadmin.SensitiveWordHandler
	productImportMappingHandler *listingadmin.ProductImportMappingHandler
	categoryHandler             *listingadmin.CategoryHandler
	productDataHandler          *listingadmin.ProductDataHandler
}

type subscriptionDependencies struct {
	subscriptionService *listingsubscription.Service
	subscriptionHandler *listingsubscription.Handler
	platformAdminUsers  []string
	platformAdminRoles  []string
}

type routeHandlerService interface {
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

func WithStoreRepository(repo listingadmin.StoreRepository) HandlerOption {
	return func(h *handler) {
		if repo != nil {
			h.storeHandler = listingadmin.NewStoreHandler(repo)
		}
	}
}

func WithStoreStatisticsRepository(repo listingadmin.StoreStatisticsRepository) HandlerOption {
	return func(h *handler) {
		if repo != nil {
			h.storeStatisticsHandler = listingadmin.NewStoreStatisticsHandler(repo)
		}
	}
}

func WithImportTaskRepository(repo listingadmin.ImportTaskRepository) HandlerOption {
	return func(h *handler) {
		if repo != nil {
			h.importTaskHandler = listingadmin.NewImportTaskHandler(repo)
		}
	}
}

func WithFilterRuleRepository(repo listingadmin.FilterRuleRepository) HandlerOption {
	return func(h *handler) {
		if repo != nil {
			h.filterRuleHandler = listingadmin.NewFilterRuleHandler(repo)
		}
	}
}

func WithProfitRuleRepository(repo listingadmin.ProfitRuleRepository) HandlerOption {
	return func(h *handler) {
		if repo != nil {
			h.profitRuleHandler = listingadmin.NewProfitRuleHandler(repo)
		}
	}
}

func WithPricingRuleRepository(repo listingadmin.PricingRuleRepository) HandlerOption {
	return func(h *handler) {
		if repo != nil {
			h.pricingRuleHandler = listingadmin.NewPricingRuleHandler(repo)
		}
	}
}

func WithOperationStrategyRepository(repo listingadmin.OperationStrategyRepository) HandlerOption {
	return func(h *handler) {
		if repo != nil {
			h.operationStrategyHandler = listingadmin.NewOperationStrategyHandler(repo)
		}
	}
}

func WithSensitiveWordRepository(repo listingadmin.SensitiveWordRepository) HandlerOption {
	return func(h *handler) {
		if repo != nil {
			h.sensitiveWordHandler = listingadmin.NewSensitiveWordHandler(repo)
		}
	}
}

func WithProductImportMappingRepository(repo listingadmin.ProductImportMappingRepository) HandlerOption {
	return func(h *handler) {
		if repo != nil {
			h.productImportMappingHandler = listingadmin.NewProductImportMappingHandler(repo)
		}
	}
}

func WithCategoryRepository(repo listingadmin.CategoryRepository) HandlerOption {
	return func(h *handler) {
		if repo != nil {
			h.categoryHandler = listingadmin.NewCategoryHandler(repo)
		}
	}
}

func WithProductDataRepository(repo listingadmin.ProductDataRepository) HandlerOption {
	return func(h *handler) {
		if repo != nil {
			h.productDataHandler = listingadmin.NewProductDataHandler(repo)
		}
	}
}

func WithSubscriptionService(service *listingsubscription.Service) HandlerOption {
	return func(h *handler) {
		if service != nil {
			h.subscriptionService = service
			h.subscriptionHandler = listingsubscription.NewHandler(service)
		}
	}
}

func WithPlatformSubscriptionAccess(users []string, roles []string) HandlerOption {
	return func(h *handler) {
		if h == nil {
			return
		}
		h.platformAdminUsers = append([]string(nil), users...)
		h.platformAdminRoles = append([]string(nil), roles...)
	}
}

func WithStudioAsyncJobStorePath(path string) HandlerOption {
	return func(h *handler) {
		if h == nil {
			return
		}
		store, err := newStudioAsyncJobStoreWithPath(path)
		if err != nil {
			h.initErr = err
			return
		}
		h.studioAsyncJobs = store
	}
}

func NewHandler(service routeHandlerService, opts ...HandlerOption) (listingkit.Handler, error) {
	if service == nil {
		return nil, errors.New("service cannot be nil")
	}
	studioAsyncJobs, err := newDefaultStudioAsyncJobStore()
	if err != nil {
		return nil, err
	}
	h := &handler{
		taskLifecycleService:  service,
		generationTaskService: service,
		studioMediaService:    service,
		storeAdminService:     service,
		studioAsyncJobs:       studioAsyncJobs,
	}
	if retryService, ok := service.(childTaskRetryService); ok {
		h.childTaskRetryService = retryService
	}
	if deleteService, ok := service.(uploadedImageDeleteService); ok {
		h.uploadedImageDeleteService = deleteService
	}
	for _, opt := range opts {
		if opt != nil {
			opt(h)
		}
	}
	h.settingsService = newSettingsService(h.storeAdminService)
	if h.initErr != nil {
		return nil, h.initErr
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
