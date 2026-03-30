package temu

import (
	"context"
	"fmt"
	"time"

	"task-processor/internal/app/taskstatus"
	"task-processor/internal/core/logger"
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
	if taskCtx.SavedToDraft {
		h.logger.WithFields(logrus.Fields{
			logger.FieldTaskID:     task.ID,
			logger.FieldDurationMs: processTime.Milliseconds(),
			logger.FieldStatus:     model.TaskStatusDraft.String(),
		}).Info("task completed with draft status")
		h.updateTaskStatusSync(task.ID, model.TaskStatusDraft, "product saved to draft")
		return nil
	}

	h.logger.WithFields(logrus.Fields{
		logger.FieldTaskID:     task.ID,
		logger.FieldDurationMs: processTime.Milliseconds(),
	}).Info("task completed")
	h.updateTaskStatusSync(task.ID, model.TaskStatusPublished, "")
	return nil
}

func (h *TaskHandler) createTaskContext(ctx context.Context, task *model.Task) *temucontext.TemuTaskContext {
	taskCtx := temucontext.NewTemuTaskContext(ctx, task)
	taskCtx.AttachRuntime(
		h.processor.GetManagementClient(),
		h.processor.GetMemoryManager(),
		nil,
	)

	h.initAPIClient(taskCtx, task)
	return taskCtx
}

func (h *TaskHandler) handleTaskFailure(task model.Task, err error) {
	if IsAuthExpiredError(err) {
		h.handleAuthenticationExpired(task, err)
		h.updateTaskStatusSync(task.ID, model.TaskStatusPaused, err.Error())
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
		h.updateTaskStatusSync(task.ID, model.TaskStatusTerminated, err.Error())
		h.logger.WithError(err).WithFields(logrus.Fields{
			logger.FieldTaskID: task.ID,
			"priority":         task.Priority,
		}).Error("task failed and is not retryable")
		return
	}

	retryDecision := model.ApplyRetryFailure(&task, h.processor.GetConfig().Processor.MaxRetries)
	if retryDecision.Exhausted {
		h.updateTaskStatusSyncWithTask(&task, model.TaskStatusTerminated, err.Error())
		h.logger.WithError(err).WithFields(logrus.Fields{
			logger.FieldTaskID:     task.ID,
			"priority":             task.Priority,
			logger.FieldRetryCount: task.RetryCount,
		}).Error("task failed after max retries")
		return
	}

	h.updateTaskStatusSyncWithTask(&task, model.TaskStatusPendingRetry, err.Error())
	h.logger.WithFields(logrus.Fields{
		logger.FieldTaskID:     task.ID,
		"old_priority":         retryDecision.OriginalPriority,
		"new_priority":         retryDecision.CurrentPriority,
		logger.FieldRetryCount: task.RetryCount,
	}).Warn("task failed and will be retried")
}

func (h *TaskHandler) isRetryableError(err error) bool {
	return IsRetryableError(err)
}

func (h *TaskHandler) handleAuthenticationExpired(task model.Task, err error) {
	managementClient := h.processor.GetManagementClient()
	if managementClient == nil {
		h.logger.WithField(logger.FieldTaskID, task.ID).Warn("management client is not initialized, skip store auth pause handling")
		return
	}

	if memoryManager := h.processor.GetMemoryManager(); memoryManager != nil {
		memoryManager.ShopPauseManager.PauseShopForDuration(
			task.TenantID,
			task.StoreID,
			"temu authentication expired",
			10*time.Minute,
		)
	}

	storeClient := managementClient.GetStoreClient()
	if storeClient == nil {
		h.logger.WithField(logger.FieldStoreID, task.StoreID).Warn("store client is not initialized, skip remote store pause")
		return
	}

	if success, pauseErr := storeClient.SetStorePauseStatus(task.StoreID, true, "auth_expired"); pauseErr != nil {
		h.logger.WithError(pauseErr).WithField(logger.FieldStoreID, task.StoreID).Warn("failed to set remote store pause status for auth expiry")
	} else if !success {
		h.logger.WithField(logger.FieldStoreID, task.StoreID).Warn("remote store pause status update returned unsuccessful for auth expiry")
	}
}

func (h *TaskHandler) updateTaskStatusSync(taskID int64, status model.TaskStatus, errorMsg string) {
	h.updateTaskStatusSyncWithInput(taskstatus.UpdateInput{
		TaskID:       taskID,
		Status:       status,
		ErrorMessage: errorMsg,
	})
}

func (h *TaskHandler) updateTaskStatusSyncWithTask(task *model.Task, status model.TaskStatus, errorMsg string) {
	if task == nil {
		return
	}

	h.updateTaskStatusSyncWithInput(taskstatus.UpdateInput{
		TaskID:       task.ID,
		Status:       status,
		ErrorMessage: errorMsg,
		RetryCount:   &task.RetryCount,
		Priority:     &task.Priority,
	})
}

func (h *TaskHandler) updateTaskStatusSyncWithInput(input taskstatus.UpdateInput) {
	h.logger.WithFields(logrus.Fields{
		logger.FieldTaskID: input.TaskID,
		logger.FieldStatus: input.Status.String(),
	}).Debug("update task status synchronously")

	statusService := taskstatus.NewService("temu/task_handler", func() taskstatus.ImportTaskStatusClient {
		managementClient := h.processor.GetManagementClient()
		if managementClient == nil {
			return nil
		}
		return managementClient.GetImportTaskClient()
	})

	if err := statusService.TransitionSyncWithInput(model.TaskStatusProcessing, input); err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			logger.FieldTaskID: input.TaskID,
			logger.FieldStatus: input.Status.String(),
		}).Error("failed to update task status")
	}
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
