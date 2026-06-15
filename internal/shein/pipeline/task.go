package pipeline

import (
	"context"
	"fmt"

	"task-processor/internal/core/logger"
	"task-processor/internal/core/metrics"
	managementAPI "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	"task-processor/internal/processor"
	sharedtenantctx "task-processor/internal/shared/tenantctx"
	shein "task-processor/internal/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheincategory "task-processor/internal/shein/api/category"
	sheinimage "task-processor/internal/shein/api/image"
	sheinother "task-processor/internal/shein/api/other"
	sheinpricing "task-processor/internal/shein/api/pricing"
	sheinproduct "task-processor/internal/shein/api/product"
	sheintranslate "task-processor/internal/shein/api/translate"
	sheinwarehouse "task-processor/internal/shein/api/warehouse"
	"task-processor/internal/shein/authorizedbrand"
	"task-processor/internal/shein/client"
	sheincontext "task-processor/internal/shein/context"
	sheinmanagedclient "task-processor/internal/shein/managedclient"

	"github.com/sirupsen/logrus"
)

type TaskHandler struct {
	*processor.BaseTaskHandler
	processor    *SheinProcessor
	errorHandler *TaskErrorHandler
	errorRouter  *TaskErrorRouter
}

type authorizedBrandResolver interface {
	Resolve(ctx context.Context, cfg authorizedbrand.Config) (*authorizedbrand.Resolved, error)
}

func NewTaskHandler(proc *SheinProcessor) *TaskHandler {
	baseHandler := processor.NewBaseTaskHandler(proc, "SHEIN")
	errorHandler := NewTaskErrorHandler(proc)

	return &TaskHandler{
		BaseTaskHandler: baseHandler,
		processor:       proc,
		errorHandler:    errorHandler,
		errorRouter:     NewTaskErrorRouter(),
	}
}

func (h *TaskHandler) ProcessTask(ctx context.Context, task model.Task, p *Pipeline) error {
	ctx = injectTaskTenantContext(ctx, &task)

	logger.GetGlobalLogger("shein/pipeline").WithFields(logrus.Fields{
		"task_id":         task.ID,
		"tenant_id":       task.TenantID,
		"store_id":        task.StoreID,
		"product_id":      task.ProductID,
		"target_platform": task.Platform,
		"source_platform": task.GetSourcePlatformOrDefault(),
	}).Info("start task")

	taskCtx := h.createTaskContext(ctx, &task)
	defer h.rollbackReservedDailyQuota(taskCtx)
	if taskCtx.InitError != nil {
		logger.GetGlobalLogger("shein/pipeline").Errorf("task initialization failed: %v", taskCtx.InitError)
		h.handleError(taskCtx, taskCtx.InitError)
		return taskCtx.InitError
	}

	if err := p.Process(taskCtx); err != nil {
		if handledErr, ok := shein.AsTaskHandledError(err); ok {
			h.handleHandledStatus(task, handledErr)
			return nil
		}
		logger.GetGlobalLogger("shein/pipeline").Errorf("task processing failed: %v", err)
		h.handleError(taskCtx, err)
		return err
	}

	h.handleSuccess(task)
	return nil
}

func injectTaskTenantContext(ctx context.Context, task *model.Task) context.Context {
	if task == nil || task.TenantID == 0 {
		return ctx
	}
	return sharedtenantctx.WithTenantID(ctx, fmt.Sprintf("%d", task.TenantID))
}

func (h *TaskHandler) rollbackReservedDailyQuota(taskCtx *sheincontext.TaskContext) {
	if taskCtx == nil || !taskCtx.DailyQuotaReserved || taskCtx.Task == nil || taskCtx.MemoryManager == nil {
		return
	}

	if taskCtx.SheinResponse != nil && len(taskCtx.SheinResponse.Info.SKCList) > 0 {
		return
	}

	if _, err := taskCtx.MemoryManager.DailyCountManager.RollbackReservedQuota(
		taskCtx.Task.TenantID,
		taskCtx.Task.StoreID,
		taskCtx.DailyQuotaDate,
		taskCtx.DailyQuotaIncrement,
	); err != nil {
		logger.GetGlobalLogger("shein/pipeline").WithError(err).Warnf(
			"failed to rollback reserved daily quota: task_id=%d, store_id=%d",
			taskCtx.Task.ID,
			taskCtx.Task.StoreID,
		)
		return
	}

	taskCtx.ClearDailyQuotaReservation()
}

func (h *TaskHandler) createTaskContext(ctx context.Context, task *model.Task) *sheincontext.TaskContext {
	taskCtx := sheincontext.NewTaskContext(ctx, task)
	taskCtx.AttachRuntime(
		h.processor.GetMemoryManager(),
		h.processor.GetManagementClient(),
		h.processor.GetAICache(),
	)

	if err := h.initShopClient(taskCtx); err != nil {
		logrus.WithError(err).Error("shop client initialization failed")
		taskCtx.InitError = err
		return taskCtx
	}

	return taskCtx
}

func (h *TaskHandler) initShopClient(taskCtx *sheincontext.TaskContext) error {
	if taskCtx.Task == nil || taskCtx.Task.StoreID == 0 {
		err := fmt.Errorf("task info or store id is empty")
		logger.GetGlobalLogger("shein/pipeline").Warn(err.Error())
		return err
	}

	managementClient := h.processor.GetManagementClient()
	if managementClient == nil {
		err := fmt.Errorf("management client is not initialized")
		logger.GetGlobalLogger("shein/pipeline").Error(err.Error())
		return err
	}

	logger.GetGlobalLogger("shein/pipeline").WithFields(logrus.Fields{
		"tenantID": taskCtx.Task.TenantID,
		"storeID":  taskCtx.Task.StoreID,
	}).Debug("initialize SHEIN shop client")

	var storeInfo *managementAPI.StoreRespDTO
	storeClient := managementClient.GetStoreClient()
	if storeClient != nil {
		if info, err := storeClient.GetStore(taskCtx.Task.StoreID); err != nil {
			logrus.WithError(err).Warn("failed to load store info, using default config")
		} else {
			storeInfo = info
			taskCtx.StoreInfo = storeInfo
		}
	}

	apiClient := sheinmanagedclient.NewAPIClientWithStoreInfo(taskCtx.Task.StoreID, managementClient, storeInfo)
	baseAPIClient := client.NewBaseAPIClient(
		apiClient.GetBaseURL(),
		apiClient.GetTenantID(),
		taskCtx.Task.StoreID,
		apiClient.GetHTTPClient(),
	)

	taskCtx.ProductAPI = sheinproduct.NewClient(baseAPIClient)
	taskCtx.CategoryAPI = sheincategory.NewClient(baseAPIClient)
	taskCtx.AttributeAPI = sheinattribute.NewClient(baseAPIClient)
	taskCtx.WarehouseAPI = sheinwarehouse.NewClient(baseAPIClient)
	taskCtx.TranslateAPI = sheintranslate.NewClient(baseAPIClient)
	taskCtx.PricingAPI = sheinpricing.NewClient(baseAPIClient)
	taskCtx.ImageAPI = sheinimage.NewClient(baseAPIClient)
	taskCtx.OtherAPI = sheinother.NewClient(baseAPIClient)

	if apiClient.HasCookies() {
		logger.GetGlobalLogger("shein/pipeline").WithFields(logrus.Fields{
			"tenantID":    taskCtx.Task.TenantID,
			"storeID":     taskCtx.Task.StoreID,
			"cookieCount": apiClient.GetCookieCount(),
		}).Info("SHEIN API client initialized with cookies")
	} else {
		logger.GetGlobalLogger("shein/pipeline").WithFields(logrus.Fields{
			"tenantID": taskCtx.Task.TenantID,
			"storeID":  taskCtx.Task.StoreID,
		}).Error("SHEIN API client initialization failed because cookies were not loaded")

		var cookieError string
		if storeClient != nil {
			if cookieStr, err := storeClient.GetStoreCookie(taskCtx.Task.StoreID); err != nil {
				cookieError = fmt.Sprintf("failed to load cookie from management service: %v", err)
			} else if cookieStr == "" {
				cookieError = "management service returned no cookie data"
			} else {
				cookieError = "cookie data exists but failed to parse or apply"
			}
		} else {
			cookieError = "store client is not initialized"
		}

		return shein.NewCookieLoadError(taskCtx.Task.TenantID, taskCtx.Task.StoreID, cookieError)
	}

	if err := applyAuthorizedBrandConfig(taskCtx, authorizedbrand.NewResolver(taskCtx.ProductAPI)); err != nil {
		return err
	}

	return nil
}

func applyAuthorizedBrandConfig(taskCtx *sheincontext.TaskContext, resolver authorizedBrandResolver) error {
	if taskCtx == nil {
		return nil
	}

	storeCfg := authorizedbrand.ConfigFromStore(taskCtx.StoreInfo)
	if !storeCfg.Enabled {
		return nil
	}

	resolved, err := resolver.Resolve(taskCtx.Context, storeCfg)
	if err != nil {
		return err
	}

	taskCtx.SetAuthorizedBrand(resolved)
	taskCtx.Context = authorizedbrand.WithResolved(taskCtx.Context, resolved)
	return nil
}

func (h *TaskHandler) handleError(taskCtx *sheincontext.TaskContext, err error) {
	task := model.Task{}
	if taskCtx != nil && taskCtx.Task != nil {
		task = *taskCtx.Task
	}

	logger.GetGlobalLogger("shein/pipeline").Infof("handle error: type=%T, value=%v", err, err)
	decision := h.errorRouter.Route(task, err)
	stage := ""
	if taskCtx != nil {
		stage = taskCtx.GetStage()
	}

	switch decision.route {
	case taskErrorRouteAuthenticationExpired:
		logger.GetGlobalLogger("shein/pipeline").Warnf("detected authentication expired error: %v", decision.authErr)
		h.errorHandler.HandleAuthenticationExpired(decision.authErr, task, stage)
	default:
		logger.GetGlobalLogger("shein/pipeline").Debugf("treating as generic error: %v", decision.err)
		h.errorHandler.HandleTaskFailure(task, decision.err, stage)
	}
}

func (h *TaskHandler) handleSuccess(task model.Task) {
	statusUpdater := NewTaskStatusUpdater(h.processor)
	statusUpdater.UpdateTaskStatusAsyncWithTask(&task, model.TaskStatusPublished, "")
	metrics.GlobalTaskMetrics().IncrementCompleted()
	metrics.GlobalSheinMetrics().IncrementPublishedForStore(task.TenantID, task.StoreID)

	logger.GetGlobalLogger("shein/pipeline").Infof(
		"task completed: id=%d, tenant_id=%d, store_id=%d, product_id=%s, target_platform=%s, source_platform=%s",
		task.ID,
		task.TenantID,
		task.StoreID,
		task.ProductID,
		task.Platform,
		task.GetSourcePlatformOrDefault(),
	)
}

func (h *TaskHandler) handleHandledStatus(task model.Task, handledErr *shein.TaskHandledError) {
	if handledErr == nil {
		return
	}

	targetStatus := handledErr.TargetStatus()
	if !targetStatus.IsValid() {
		logger.GetGlobalLogger("shein/pipeline").Warnf(
			"task handled with invalid target status: id=%d, status=%d, message=%s",
			task.ID,
			targetStatus,
			handledErr.Error(),
		)
		return
	}

	statusUpdater := NewTaskStatusUpdater(h.processor)
	statusUpdater.UpdateTaskStatusAsyncWithTask(&task, targetStatus, handledErr.ErrorMessage())
	metrics.GlobalSheinMetrics().IncrementHandledStatusForStore(task.TenantID, task.StoreID, targetStatus)
	if reasonCode := metrics.ExtractReasonCode(handledErr.ErrorMessage()); reasonCode != "" {
		metrics.GlobalSheinMetrics().IncrementReasonForStore(task.TenantID, task.StoreID, reasonCode)
	} else if reasonCode := metrics.ExtractReasonCode(handledErr.Error()); reasonCode != "" {
		metrics.GlobalSheinMetrics().IncrementReasonForStore(task.TenantID, task.StoreID, reasonCode)
	}

	logger.GetGlobalLogger("shein/pipeline").Infof(
		"task finished with handled status: id=%d, status=%s, reason=%s, target_platform=%s, source_platform=%s",
		task.ID,
		targetStatus.String(),
		handledErr.Error(),
		task.Platform,
		task.GetSourcePlatformOrDefault(),
	)
}
