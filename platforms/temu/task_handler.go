package temu

import (
	"context"
	"task-processor/common/pipeline"
	"task-processor/common/temu"
	"task-processor/common/types"
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

// ProcessTask 处理任务
func (h *TaskHandler) ProcessTask(ctx context.Context, task types.Task, pipeline *pipeline.Pipeline) error {
	h.logger.Infof("开始处理任务: ID=%s, ProductID=%s, Platform=%s", task.ID, task.ProductID, task.Platform)

	// 创建任务上下文
	taskCtx := h.createTaskContext(ctx, &task)

	// 记录开始时间
	startTime := time.Now()

	// 执行管道处理
	if err := pipeline.Process(taskCtx); err != nil {
		h.logger.Errorf("任务处理失败: %v", err)
		h.handleTaskFailure(task, err)
		return err
	}

	// 记录处理时间
	processTime := time.Since(startTime)
	h.logger.Infof("任务处理成功: ID=%s, 耗时=%v", task.ID, processTime)

	// 更新任务状态为已完成
	h.updateTaskStatus(task.ID, "completed", "")

	return nil
}

// createTaskContext 创建任务上下文
func (h *TaskHandler) createTaskContext(ctx context.Context, task *types.Task) *pipeline.TaskContext {
	taskCtx := pipeline.NewTaskContext(ctx, task)

	// 设置Amazon处理器
	if h.processor.amazonProcessor != nil {
		taskCtx.SetAmazonProcessor(h.processor.amazonProcessor)
	}

	// 初始化并设置TEMU API客户端
	h.initAPIClient(taskCtx, task)

	return taskCtx
}

// handleTaskFailure 处理任务失败
func (h *TaskHandler) handleTaskFailure(task types.Task, err error) {
	isRetryable := h.isRetryableError(err)
	h.logger.Infof("错误类型: %T, 错误值: %v, 是否可重试: %t", err, err, isRetryable)

	if !isRetryable {
		h.updateTaskStatus(task.ID, "terminated", err.Error())
		h.logger.Errorf("任务处理失败且不可重试: ID=%s, Priority=%d, 错误=%v", task.ID, task.Priority, err)
		return
	}

	task.RetryCount++
	originalPriority := task.Priority

	// 降低优先级
	if task.RetryCount > 0 && task.Priority > 10 {
		task.Priority = task.Priority - 10
		if task.Priority < 0 {
			task.Priority = 0
		}
	}

	maxRetries := h.processor.config.Processor.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3 // 默认最大重试次数
	}

	if task.RetryCount >= maxRetries {
		h.updateTaskStatus(task.ID, "terminated", err.Error())
		h.logger.Errorf("任务处理失败且达到最大重试次数: ID=%s, Priority=%d, 重试次数=%d, 错误=%v",
			task.ID, task.Priority, task.RetryCount, err)
	} else {
		h.updateTaskStatus(task.ID, "pending_retry", err.Error())
		h.logger.Warnf("任务处理失败，等待重试: ID=%s, Priority=%d->%d, 重试次数=%d",
			task.ID, originalPriority, task.Priority, task.RetryCount)
	}
}

// isRetryableError 判断错误是否可重试
func (h *TaskHandler) isRetryableError(err error) bool {
	return IsRetryableError(err)
}

// updateTaskStatus 更新任务状态
func (h *TaskHandler) updateTaskStatus(taskID, status, errorMsg string) {
	// 这里应该调用管理系统API更新任务状态
	// 目前只记录日志
	h.logger.Infof("更新任务状态: TaskID=%s, Status=%s, Error=%s", taskID, status, errorMsg)
}

// initAPIClient 初始化API客户端
func (h *TaskHandler) initAPIClient(taskCtx *pipeline.TaskContext, task *types.Task) {
	// 从任务中获取租户ID和店铺ID
	tenantID := int64(1) // 默认租户ID，实际应该从任务或配置中获取
	storeID := task.StoreID

	// 获取管理系统客户端
	managementClient := h.processor.GetManagementClient()
	if managementClient == nil {
		h.logger.Error("管理系统客户端未初始化")
		return
	}

	h.logger.WithFields(logrus.Fields{
		"tenantID": tenantID,
		"storeID":  storeID,
	}).Debug("开始初始化API客户端")

	// 创建API客户端，会自动加载Cookie
	apiClient := temu.NewAPIClient(tenantID, storeID, managementClient)

	// 检查cookie加载状态
	if apiClient.HasCookies() {
		h.logger.WithFields(logrus.Fields{
			"tenantID":    tenantID,
			"storeID":     storeID,
			"cookieCount": apiClient.GetCookieCount(),
		}).Info("API客户端初始化成功，已加载Cookie")
	} else {
		h.logger.WithFields(logrus.Fields{
			"tenantID": tenantID,
			"storeID":  storeID,
		}).Warn("API客户端初始化完成，但未加载到Cookie")
	}

	// 设置到任务上下文
	taskCtx.SetAPIClient(apiClient)
}
