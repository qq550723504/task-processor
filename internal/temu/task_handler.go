package temu

import (
	"context"
	"fmt"
	"time"

	"task-processor/internal/core/logger"
	managementAPI "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	"task-processor/internal/temu/api"
	temucontext "task-processor/internal/temu/context"

	"github.com/sirupsen/logrus"
)

type TaskHandler struct {
	processor *TemuProcessor
	logger    *logrus.Entry
	helper    *logger.LoggerHelper
}

func NewTaskHandler(processor *TemuProcessor) *TaskHandler {
	log := logger.GetGlobalLogger("task_handler").WithFields(logrus.Fields{
		logger.FieldComponent: "task_handler",
		logger.FieldPlatform:  "temu",
	})

	return &TaskHandler{
		processor: processor,
		logger:    log,
		helper:    logger.NewLoggerHelper(log),
	}
}

func (h *TaskHandler) ProcessTask(ctx context.Context, task model.Task, executor *TemuPipelineExecutor) error {
	h.logger.WithFields(logrus.Fields{
		logger.FieldTaskID:    task.ID,
		logger.FieldProductID: task.ProductID,
	}).Info("start task")

	taskCtx := h.createTaskContext(ctx, &task)
	startTime := time.Now()

	if err := executor.Execute(taskCtx); err != nil {
		h.logger.WithError(err).WithField(logger.FieldTaskID, task.ID).Error("task failed")
		h.handleTaskFailure(task, err)
		return err
	}

	processTime := time.Since(startTime)
	savedToDraft := taskCtx.SavedToDraft

	if savedToDraft {
		h.logger.WithFields(logrus.Fields{
			logger.FieldTaskID:     task.ID,
			logger.FieldDurationMs: processTime.Milliseconds(),
			logger.FieldStatus:     "draft",
		}).Info("task completed with draft status")
		h.updateTaskStatusSync(task.ID, "draft", "product saved to draft")
	} else {
		h.logger.WithFields(logrus.Fields{
			logger.FieldTaskID:     task.ID,
			logger.FieldDurationMs: processTime.Milliseconds(),
		}).Info("task completed")
		h.updateTaskStatusSync(task.ID, "completed", "")
	}

	return nil
}

func (h *TaskHandler) createTaskContext(ctx context.Context, task *model.Task) *temucontext.TemuTaskContext {
	taskCtx := temucontext.NewTemuTaskContext(ctx, task)
	taskCtx.AttachRuntime(
		h.processor.GetManagementClient(),
		h.processor.GetMemoryManager(),
		h.processor.amazonProcessor,
	)

	h.initAPIClient(taskCtx, task)
	return taskCtx
}

func (h *TaskHandler) handleTaskFailure(task model.Task, err error) {
	if IsAuthExpiredError(err) {
		h.updateTaskStatusSync(task.ID, "paused", err.Error())
		h.logger.WithFields(logrus.Fields{
			logger.FieldTaskID:  task.ID,
			logger.FieldStoreID: task.StoreID,
		}).Warn("task paused because authentication expired")
		return
	}

	isRetryable := h.isRetryableError(err)
	h.logger.WithFields(logrus.Fields{
		"error_type": fmt.Sprintf("%T", err),
		"retryable":  isRetryable,
	}).Debug("error analysis")

	if !isRetryable {
		h.updateTaskStatusSync(task.ID, "terminated", err.Error())
		h.logger.WithError(err).WithFields(logrus.Fields{
			logger.FieldTaskID: task.ID,
			"priority":         task.Priority,
		}).Error("task failed and is not retryable")
		return
	}

	task.RetryCount++
	originalPriority := task.Priority

	if task.RetryCount > 0 && task.Priority > 10 {
		task.Priority = max(0, task.Priority-10)
	}

	maxRetries := h.processor.GetConfig().Processor.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	if task.RetryCount >= maxRetries {
		h.updateTaskStatusSync(task.ID, "terminated", err.Error())
		h.logger.WithError(err).WithFields(logrus.Fields{
			logger.FieldTaskID:     task.ID,
			"priority":             task.Priority,
			logger.FieldRetryCount: task.RetryCount,
		}).Error("task failed after max retries")
	} else {
		h.updateTaskStatusSync(task.ID, "pending_retry", err.Error())
		h.logger.WithFields(logrus.Fields{
			logger.FieldTaskID:     task.ID,
			"old_priority":         originalPriority,
			"new_priority":         task.Priority,
			logger.FieldRetryCount: task.RetryCount,
		}).Warn("task failed and will be retried")
	}
}

func (h *TaskHandler) isRetryableError(err error) bool {
	return IsRetryableError(err)
}

func (h *TaskHandler) updateTaskStatusSync(taskID int64, status, errorMsg string) {
	h.logger.WithFields(logrus.Fields{
		logger.FieldTaskID: taskID,
		logger.FieldStatus: status,
	}).Debug("update task status synchronously")

	managementClient := h.processor.GetManagementClient()
	if managementClient == nil {
		h.logger.WithField(logger.FieldTaskID, taskID).Error("management client is not initialized")
		return
	}

	importTaskClient := managementClient.GetImportTaskClient()
	if importTaskClient == nil {
		h.logger.WithField(logger.FieldTaskID, taskID).Error("import task client is not initialized")
		return
	}

	var statusCode int16
	switch status {
	case "completed":
		statusCode = model.TaskStatusPublished.Int16()
	case "draft":
		statusCode = model.TaskStatusDraft.Int16()
	case "pending_retry":
		statusCode = model.TaskStatusPendingRetry.Int16()
	case "terminated":
		statusCode = model.TaskStatusTerminated.Int16()
	case "paused":
		statusCode = model.TaskStatusPaused.Int16()
	default:
		h.logger.WithFields(logrus.Fields{
			logger.FieldTaskID: taskID,
			logger.FieldStatus: status,
		}).Warn("unknown task status, using pending_retry")
		statusCode = model.TaskStatusPendingRetry.Int16()
	}

	req := &managementAPI.ProductImportTaskUpdateReqDTO{
		ID:           taskID,
		Status:       statusCode,
		ErrorMessage: errorMsg,
	}

	const maxRetries = 3
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if err := importTaskClient.UpdateTaskStatus(req); err != nil {
			lastErr = err
			if i < maxRetries-1 {
				h.helper.LogRetry("update_task_status", i+1, maxRetries, err)
				time.Sleep(time.Second * time.Duration(i+1))
				continue
			}
		} else {
			h.logger.WithFields(logrus.Fields{
				logger.FieldTaskID: taskID,
				logger.FieldStatus: status,
			}).Info("task status updated")
			return
		}
	}

	h.logger.WithError(lastErr).WithFields(logrus.Fields{
		logger.FieldTaskID:     taskID,
		logger.FieldRetryCount: maxRetries,
	}).Error("failed to update task status")
}

func (h *TaskHandler) initAPIClient(taskCtx *temucontext.TemuTaskContext, task *model.Task) {
	storeID := task.StoreID

	managementClient := h.processor.GetManagementClient()
	if managementClient == nil {
		h.logger.WithField(logger.FieldTaskID, task.ID).Error("management client is not initialized")
		return
	}

	h.logger.WithFields(logrus.Fields{
		logger.FieldTenantID: task.TenantID,
		logger.FieldStoreID:  storeID,
	}).Debug("initialize TEMU API client")

	apiClient := api.NewAPIClient(storeID, managementClient)
	if apiClient.HasCookies() {
		h.logger.WithFields(logrus.Fields{
			logger.FieldTenantID: task.TenantID,
			logger.FieldStoreID:  storeID,
			"cookie_count":       apiClient.GetCookieCount(),
		}).Info("TEMU API client initialized with cookies")
	} else {
		h.logger.WithFields(logrus.Fields{
			logger.FieldTenantID: task.TenantID,
			logger.FieldStoreID:  storeID,
		}).Warn("TEMU API client initialized without cookies")
	}

	taskCtx.SetAPIClients(
		apiClient,
		api.NewQueryAPI(apiClient, h.logger.WithField("service", "QueryAPI")),
	)
}
