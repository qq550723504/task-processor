// Package pipeline 提供SHEIN平台的任务处理器
package pipeline

import (
	"context"
	"fmt"

	"task-processor/internal/app/processor"
	"task-processor/internal/core/metrics"
	management_api "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	"task-processor/internal/pipeline"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/api"
	shein_attribute "task-processor/internal/shein/api/attribute"
	shein_category "task-processor/internal/shein/api/category"
	shein_image "task-processor/internal/shein/api/image"
	shein_other "task-processor/internal/shein/api/other"
	shein_pricing "task-processor/internal/shein/api/pricing"
	shein_product "task-processor/internal/shein/api/product"
	shein_translate "task-processor/internal/shein/api/translate"
	shein_warehouse "task-processor/internal/shein/api/warehouse"
	"task-processor/internal/shein/client"

	"github.com/sirupsen/logrus"
)

// TaskHandler SHEIN任务处理器（统一架构版本）
type TaskHandler struct {
	*processor.BaseTaskHandler
	processor    *SheinProcessor
	errorHandler *TaskErrorHandler
}

// NewTaskHandler 创建SHEIN任务处理器
func NewTaskHandler(proc *SheinProcessor) *TaskHandler {
	baseHandler := processor.NewBaseTaskHandler(proc, "SHEIN")
	errorHandler := NewTaskErrorHandler(proc)

	return &TaskHandler{
		BaseTaskHandler: baseHandler,
		processor:       proc,
		errorHandler:    errorHandler,
	}
}

// ProcessTask 处理任务（统一接口）
func (h *TaskHandler) ProcessTask(ctx context.Context, task model.Task, pipeline pipeline.Pipeline) error {
	logrus.Infof("开始处理任务: ID=%d, ProductID=%s", task.ID, task.ProductID)

	// 创建任务上下文
	taskCtx := h.createTaskContext(ctx, &task)

	// 检查初始化是否成功
	if initError, exists := taskCtx.GetData("init_error"); exists {
		if err, ok := initError.(error); ok {
			logrus.Errorf("任务初始化失败，停止处理: %v", err)
			h.handleError(task, err)
			return err
		}
	}

	// 执行管道处理
	if err := pipeline.Process(taskCtx); err != nil {
		logrus.Errorf("任务处理失败: %v", err)
		h.handleError(task, err)
		return err
	}

	// 处理成功
	h.handleSuccess(task)
	return nil
}

// createTaskContext 创建任务上下文
func (h *TaskHandler) createTaskContext(ctx context.Context, task *model.Task) *shein.TaskContext {
	taskCtx := shein.NewTaskContext(ctx, task)

	// 设置基础组件
	taskCtx.MemoryManager = h.processor.GetMemoryManager()
	taskCtx.ManagementClientMgr = h.processor.GetManagementClient()

	// 初始化店铺客户端
	if err := h.initShopClient(taskCtx); err != nil {
		logrus.WithError(err).Error("店铺客户端初始化失败")
		// 设置错误标记，让调用方知道初始化失败
		taskCtx.SetData("init_error", err)
		return taskCtx
	}

	return taskCtx
}

// initShopClient 初始化店铺客户端（参考TEMU的cookie加载机制）
func (h *TaskHandler) initShopClient(taskCtx *shein.TaskContext) error {
	if taskCtx.Task == nil || taskCtx.Task.StoreID == 0 {
		err := fmt.Errorf("任务信息或店铺ID为空")
		logrus.Warn(err.Error())
		return err
	}

	// 获取管理系统客户端
	managementClient := h.processor.GetManagementClient()
	if managementClient == nil {
		err := fmt.Errorf("管理系统客户端未初始化")
		logrus.Error(err.Error())
		return err
	}

	logrus.WithFields(logrus.Fields{
		"tenantID": taskCtx.Task.TenantID,
		"storeID":  taskCtx.Task.StoreID,
	}).Debug("开始初始化SHEIN店铺客户端")

	// 先获取店铺信息，用于后续的端点和代理配置
	var storeInfo *management_api.StoreRespDTO
	storeClient := managementClient.GetStoreClient()
	if storeClient != nil {
		if info, err := storeClient.GetStore(taskCtx.Task.StoreID); err != nil {
			logrus.WithError(err).Warn("获取店铺信息失败，将使用默认配置")
		} else {
			storeInfo = info
			taskCtx.StoreInfo = storeInfo
		}
	}

	// 创建SHEIN API客户端（会自动根据loginUrl设置端点并加载Cookie）
	apiClient := client.NewAPIClient(
		taskCtx.Task.StoreID,
		managementClient,
	)

	// 创建BaseAPIClient用于各个API调用
	baseAPIClient := client.NewBaseAPIClient(
		apiClient.GetBaseURL(),
		apiClient.GetTenantID(),
		taskCtx.Task.StoreID,
		apiClient.GetHTTPClient(),
	)

	// 初始化各个具体的API实例
	taskCtx.ProductAPI = shein_product.NewClient(baseAPIClient)
	taskCtx.CategoryAPI = shein_category.NewClient(baseAPIClient)
	taskCtx.AttributeAPI = shein_attribute.NewClient(baseAPIClient)
	taskCtx.WarehouseAPI = shein_warehouse.NewClient(baseAPIClient)
	taskCtx.TranslateAPI = shein_translate.NewClient(baseAPIClient)
	taskCtx.PricingAPI = shein_pricing.NewClient(baseAPIClient)
	taskCtx.ImageAPI = shein_image.NewClient(baseAPIClient)
	taskCtx.OtherAPI = shein_other.NewClient(baseAPIClient)

	// 检查Cookie加载状态（参考TEMU的检查方式）
	if apiClient.HasCookies() {
		logrus.WithFields(logrus.Fields{
			"tenantID":    taskCtx.Task.TenantID,
			"storeID":     taskCtx.Task.StoreID,
			"cookieCount": apiClient.GetCookieCount(),
		}).Info("SHEIN API客户端初始化成功，已加载Cookie")
	} else {
		// Cookie加载失败，记录详细错误信息并返回错误
		logrus.WithFields(logrus.Fields{
			"tenantID": taskCtx.Task.TenantID,
			"storeID":  taskCtx.Task.StoreID,
		}).Error("SHEIN API客户端初始化失败，未加载到Cookie")

		// 获取详细的Cookie加载失败原因
		var cookieError string
		if storeClient != nil {
			if cookieStr, err := storeClient.GetStoreCookie(taskCtx.Task.StoreID); err != nil {
				cookieError = fmt.Sprintf("从管理系统获取Cookie失败: %v", err)
			} else if cookieStr == "" {
				cookieError = "管理系统中未找到Cookie数据，请检查店铺登录状态"
			} else {
				cookieError = "Cookie数据存在但解析或设置失败"
			}
		} else {
			cookieError = "店铺客户端未初始化"
		}

		return shein.NewCookieLoadError(taskCtx.Task.TenantID, taskCtx.Task.StoreID, cookieError)
	}

	return nil
}

// handleError 处理错误
func (h *TaskHandler) handleError(task model.Task, err error) {
	// 添加调试日志，查看错误的具体类型和内容
	logrus.Infof("处理错误: 类型=%T, 内容=%v", err, err)

	// 检查是否是Cookie加载失败错误
	if cookieErr, isCookieError := shein.IsCookieLoadError(err); isCookieError {
		logrus.Errorf("检测到Cookie加载失败错误: %v", cookieErr)
		// Cookie加载失败，直接处理为任务失败，不进行重试
		h.errorHandler.HandleTaskFailure(task, cookieErr)
		return
	}

	// 检查是否是认证过期错误
	if authErr, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
		logrus.Warnf("检测到认证过期错误: %v", authErr)
		h.errorHandler.HandleAuthenticationExpired(authErr, task)
		return
	}

	// 添加调试日志，显示未匹配到认证过期错误
	logrus.Debugf("未检测到认证过期错误，按一般错误处理: %v", err)

	// 处理一般错误
	h.errorHandler.HandleTaskFailure(task, err)
}

// handleSuccess 处理成功
func (h *TaskHandler) handleSuccess(task model.Task) {
	statusUpdater := NewTaskStatusUpdater(h.processor)
	statusUpdater.UpdateTaskStatusAsync(fmt.Sprintf("%d", task.ID), model.TaskStatusPublished, "")
	metrics.GlobalTaskMetrics().IncrementCompleted()

	logrus.Infof("任务处理成功: ID=%d, TenantID=%d, StoreID=%d, ProductID=%s",
		task.ID, task.TenantID, task.StoreID, task.ProductID)
}
