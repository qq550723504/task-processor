package pipeline

import (
	"context"
	"fmt"

	"task-processor/internal/app/processor"
	"task-processor/internal/core/logger"
	"task-processor/internal/core/metrics"
	managementAPI "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/api"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheincategory "task-processor/internal/shein/api/category"
	sheinimage "task-processor/internal/shein/api/image"
	sheinother "task-processor/internal/shein/api/other"
	sheinpricing "task-processor/internal/shein/api/pricing"
	sheinproduct "task-processor/internal/shein/api/product"
	sheintranslate "task-processor/internal/shein/api/translate"
	sheinwarehouse "task-processor/internal/shein/api/warehouse"
	"task-processor/internal/shein/client"
	sheincontext "task-processor/internal/shein/context"

	"github.com/sirupsen/logrus"
)

type TaskHandler struct {
	*processor.BaseTaskHandler
	processor    *SheinProcessor
	errorHandler *TaskErrorHandler
}

func NewTaskHandler(proc *SheinProcessor) *TaskHandler {
	baseHandler := processor.NewBaseTaskHandler(proc, "SHEIN")
	errorHandler := NewTaskErrorHandler(proc)

	return &TaskHandler{
		BaseTaskHandler: baseHandler,
		processor:       proc,
		errorHandler:    errorHandler,
	}
}

func (h *TaskHandler) ProcessTask(ctx context.Context, task model.Task, p *Pipeline) error {
	logger.GetGlobalLogger("shein/pipeline").Infof("start task: id=%d, product_id=%s", task.ID, task.ProductID)

	taskCtx := h.createTaskContext(ctx, &task)
	if taskCtx.InitError != nil {
		logger.GetGlobalLogger("shein/pipeline").Errorf("task initialization failed: %v", taskCtx.InitError)
		h.handleError(task, taskCtx.InitError)
		return taskCtx.InitError
	}

	if err := p.Process(taskCtx); err != nil {
		logger.GetGlobalLogger("shein/pipeline").Errorf("task processing failed: %v", err)
		h.handleError(task, err)
		return err
	}

	h.handleSuccess(task)
	return nil
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

	apiClient := client.NewAPIClient(taskCtx.Task.StoreID, managementClient)
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

	return nil
}

func (h *TaskHandler) handleError(task model.Task, err error) {
	logger.GetGlobalLogger("shein/pipeline").Infof("handle error: type=%T, value=%v", err, err)

	if cookieErr, isCookieError := shein.IsCookieLoadError(err); isCookieError {
		logger.GetGlobalLogger("shein/pipeline").Errorf("detected cookie load error: %v", cookieErr)
		h.errorHandler.HandleTaskFailure(task, cookieErr)
		return
	}

	if authErr, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
		logger.GetGlobalLogger("shein/pipeline").Warnf("detected authentication expired error: %v", authErr)
		h.errorHandler.HandleAuthenticationExpired(authErr, task)
		return
	}

	logger.GetGlobalLogger("shein/pipeline").Debugf("treating as generic error: %v", err)
	h.errorHandler.HandleTaskFailure(task, err)
}

func (h *TaskHandler) handleSuccess(task model.Task) {
	statusUpdater := NewTaskStatusUpdater(h.processor)
	statusUpdater.UpdateTaskStatusAsync(fmt.Sprintf("%d", task.ID), model.TaskStatusPublished, "")
	metrics.GlobalTaskMetrics().IncrementCompleted()

	logger.GetGlobalLogger("shein/pipeline").Infof(
		"task completed: id=%d, tenant_id=%d, store_id=%d, product_id=%s",
		task.ID,
		task.TenantID,
		task.StoreID,
		task.ProductID,
	)
}
