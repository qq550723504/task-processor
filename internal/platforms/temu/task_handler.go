package temu

import (
	"context"
	management_api "task-processor/internal/common/management/api"
	"task-processor/internal/common/temu"
	"task-processor/internal/model"
	temucontext "task-processor/internal/platforms/temu/context"
	"time"

	"github.com/sirupsen/logrus"
)

// TaskHandler TEMU任务处理器
type TaskHandler struct {
	processor *TemuProcessor
	logger    *logrus.Entry
}

// NewTaskHandler 创建TEMU任务处理器
func NewTaskHandler(processor *TemuProcessor) *TaskHandler {
	return &TaskHandler{
		processor: processor,
		logger: logrus.WithFields(logrus.Fields{
			"component": "TaskHandler",
			"platform":  "temu",
		}),
	}
}

// ProcessTask 处理任务（使用强类型管道执行器）
func (h *TaskHandler) ProcessTask(ctx context.Context, task model.Task, executor *TemuPipelineExecutor) error {
	h.logger.Infof("开始处理任务: ID=%d, ProductID=%s", task.ID, task.ProductID)

	// 创建任务上下文
	taskCtx := h.createTaskContext(ctx, &task)

	// 记录开始时间
	startTime := time.Now()

	// 直接使用传入的强类型执行器
	if err := executor.Execute(taskCtx); err != nil {
		h.logger.Errorf("任务处理失败: %v", err)
		h.handleTaskFailure(task, err)
		return err
	}

	// 记录处理时间
	processTime := time.Since(startTime)

	// 检查是否保存到了草稿箱
	savedToDraft := false
	if draftFlag, exists := taskCtx.GetData("saved_to_draft"); exists {
		if flag, ok := draftFlag.(bool); ok && flag {
			savedToDraft = true
		}
	}

	if savedToDraft {
		h.logger.Infof("任务处理完成(已保存到草稿箱): ID=%d, 耗时=%v", task.ID, processTime)
		// 同步更新任务状态为草稿箱，确保状态立即生效，避免重复处理
		h.updateTaskStatusSync(task.ID, "draft", "产品已保存到草稿箱")
	} else {
		h.logger.Infof("任务处理成功: ID=%d, 耗时=%v", task.ID, processTime)
		// 同步更新任务状态为已完成，确保状态立即生效，避免重复处理
		h.updateTaskStatusSync(task.ID, "completed", "")
	}

	return nil
}

// createTaskContext 创建任务上下文
func (h *TaskHandler) createTaskContext(ctx context.Context, task *model.Task) *temucontext.TemuTaskContext {
	// 直接使用新的context包中的TemuTaskContext
	taskCtx := temucontext.NewTemuTaskContext(ctx, task)

	// 设置管理客户端和内存管理器
	taskCtx.SetManagementClient(h.processor.GetManagementClient())
	taskCtx.SetMemoryManager(h.processor.GetMemoryManager())

	// 设置Amazon处理器
	if h.processor.amazonProcessor != nil {
		taskCtx.AmazonProcessor = h.processor.amazonProcessor
	}

	// 初始化并设置TEMU API客户端
	h.initAPIClient(taskCtx, task)

	return taskCtx
}

// handleTaskFailure 处理任务失败
func (h *TaskHandler) handleTaskFailure(task model.Task, err error) {
	// 首先检查是否为认证过期错误（Cookie为空）
	isAuthExpired := IsAuthExpiredError(err)
	if isAuthExpired {
		// 认证过期错误，暂停任务等待Cookie更新
		h.updateTaskStatusSync(task.ID, "paused", err.Error())
		h.logger.Warnf("⏸️ 任务因认证过期而暂停: ID=%d, StoreID=%d", task.ID, task.StoreID)
		return
	}

	isRetryable := h.isRetryableError(err)
	h.logger.Debugf("错误分析: 类型=%T, 可重试=%t", err, isRetryable)

	if !isRetryable {
		// 不可重试错误，同步更新状态确保立即生效
		h.updateTaskStatusSync(task.ID, "terminated", err.Error())
		h.logger.Errorf("❌ 任务处理失败且不可重试: ID=%d, Priority=%d, 错误=%v", task.ID, task.Priority, err)
		return
	}

	task.RetryCount++
	originalPriority := task.Priority

	// 降低优先级
	if task.RetryCount > 0 && task.Priority > 10 {
		task.Priority = max(0, task.Priority-10)
	}

	maxRetries := h.processor.GetConfig().Processor.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3 // 默认最大重试次数
	}

	if task.RetryCount >= maxRetries {
		// 达到最大重试次数，同步更新状态
		h.updateTaskStatusSync(task.ID, "terminated", err.Error())
		h.logger.Errorf("❌ 任务处理失败且达到最大重试次数: ID=%d, Priority=%d, 重试次数=%d, 错误=%v",
			task.ID, task.Priority, task.RetryCount, err)
	} else {
		// 待重试，同步更新状态避免重复获取
		h.updateTaskStatusSync(task.ID, "pending_retry", err.Error())
		h.logger.Warnf("⚠️ 任务处理失败，等待重试: ID=%d, Priority=%d->%d, 重试次数=%d",
			task.ID, originalPriority, task.Priority, task.RetryCount)
	}
}

// isRetryableError 判断错误是否可重试
func (h *TaskHandler) isRetryableError(err error) bool {
	return IsRetryableError(err)
}

// updateTaskStatusSync 同步更新任务状态
func (h *TaskHandler) updateTaskStatusSync(taskID int64, status, errorMsg string) {
	h.logger.Debugf("准备同步更新任务状态: TaskID=%d, Status=%s", taskID, status)

	// 获取管理系统客户端
	managementClient := h.processor.GetManagementClient()
	if managementClient == nil {
		h.logger.Error("管理系统客户端未初始化，无法更新任务状态")
		return
	}

	// 获取导入任务客户端
	importTaskClient := managementClient.GetImportTaskClient()
	if importTaskClient == nil {
		h.logger.Error("导入任务客户端未初始化，无法更新任务状态")
		return
	}

	// 映射状态到int16类型
	var statusCode int16
	switch status {
	case "completed":
		statusCode = model.TaskStatusPublished.Int16() // 已发布
	case "draft":
		statusCode = model.TaskStatusDraft.Int16() // 草稿箱
	case "pending_retry":
		statusCode = model.TaskStatusPendingRetry.Int16() // 待重试
	case "terminated":
		statusCode = model.TaskStatusTerminated.Int16() // 已终止
	case "paused":
		statusCode = model.TaskStatusPaused.Int16() // 已暂停
	default:
		h.logger.Warnf("未知的任务状态: %s，使用默认状态", status)
		statusCode = model.TaskStatusPendingRetry.Int16()
	}

	// 构建更新请求
	req := &management_api.ProductImportTaskUpdateReqDTO{
		ID:           taskID,
		Status:       statusCode,
		ErrorMessage: errorMsg,
	}

	// 同步更新状态，带重试机制
	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if err := importTaskClient.UpdateTaskStatus(req); err != nil {
			lastErr = err
			if i < maxRetries-1 {
				h.logger.Warnf("同步更新任务状态失败，将重试 (%d/%d): TaskID=%d, Error=%v",
					i+1, maxRetries, taskID, err)
				time.Sleep(time.Second * time.Duration(i+1)) // 指数退避
				continue
			}
		} else {
			h.logger.Infof("✅ 任务状态同步更新成功: TaskID=%d, Status=%s", taskID, status)
			return
		}
	}

	h.logger.Errorf("❌ 同步更新任务状态失败，已重试%d次: TaskID=%d, Error=%v",
		maxRetries, taskID, lastErr)
}

// initAPIClient 初始化API客户端
func (h *TaskHandler) initAPIClient(taskCtx *temucontext.TemuTaskContext, task *model.Task) {
	// 从任务中获取租户ID和店铺ID
	storeID := task.StoreID

	// 获取管理系统客户端
	managementClient := h.processor.GetManagementClient()
	if managementClient == nil {
		h.logger.Error("管理系统客户端未初始化")
		return
	}

	h.logger.WithFields(logrus.Fields{
		"tenantID": task.TenantID,
		"storeID":  storeID,
	}).Debug("开始初始化API客户端")

	// 创建API客户端，会自动加载Cookie
	apiClient := temu.NewAPIClient(task.TenantID, storeID, managementClient)

	// 检查cookie加载状态
	if apiClient.HasCookies() {
		h.logger.WithFields(logrus.Fields{
			"tenantID":    task.TenantID,
			"storeID":     storeID,
			"cookieCount": apiClient.GetCookieCount(),
		}).Info("API客户端初始化成功，已加载Cookie")
	} else {
		h.logger.WithFields(logrus.Fields{
			"tenantID": task.TenantID,
			"storeID":  storeID,
		}).Warn("API客户端初始化完成，但未加载到Cookie")
	}

	// 设置到任务上下文（直接赋值）
	taskCtx.APIClient = apiClient
}
