package amazon

import (
	"strings"
	"time"

	amazonModel "task-processor/internal/amazon/model"
	"task-processor/internal/app/taskstatus"
	"task-processor/internal/model"
	"task-processor/internal/pkg/timex"
	"task-processor/internal/state"

	"github.com/sirupsen/logrus"
)

const (
	amazonTaskReasonAuthExpired       = "AUTH_EXPIRED"
	amazonTaskReasonDailyLimitReached = "DAILY_LIMIT_REACHED"
	amazonTaskStagePublishCompleted   = "publish_completed"
)

func (p *Processor) handleTaskFailure(task *model.Task, err error) {
	if task == nil || err == nil {
		return
	}

	reasonCode, stage := parseTaskStatusMetadata(err.Error())
	if stage == "" {
		stage = amazonTaskStagePublishCompleted
	}

	if shouldPauseAmazonTask(reasonCode) {
		if reasonCode == amazonTaskReasonAuthExpired {
			p.pauseStoreForAuthentication(task)
		}
		p.updateTaskStatusSyncWithInput(taskstatus.UpdateInput{
			TaskID:       task.ID,
			Status:       model.TaskStatusPaused,
			ErrorMessage: err.Error(),
			ReasonCode:   reasonCode,
			Stage:        stage,
		})
		return
	}

	if !isAmazonRetryableTaskError(err) {
		p.updateTaskStatusSyncWithInput(taskstatus.UpdateInput{
			TaskID:       task.ID,
			Status:       model.TaskStatusTerminated,
			ErrorMessage: err.Error(),
			ReasonCode:   reasonCode,
			Stage:        stage,
		})
		return
	}

	retryDecision := model.ApplyRetryFailure(task, p.GetConfig().Processor.MaxRetries)
	if retryDecision.Exhausted {
		p.updateTaskStatusSyncWithInput(taskstatus.UpdateInput{
			TaskID:       task.ID,
			Status:       model.TaskStatusTerminated,
			ErrorMessage: err.Error(),
			ReasonCode:   reasonCode,
			Stage:        stage,
			RetryCount:   &task.RetryCount,
			Priority:     &task.Priority,
		})
		return
	}

	p.updateTaskStatusSyncWithInput(taskstatus.UpdateInput{
		TaskID:       task.ID,
		Status:       model.TaskStatusPendingRetry,
		ErrorMessage: err.Error(),
		ReasonCode:   reasonCode,
		Stage:        stage,
		RetryCount:   &task.RetryCount,
		Priority:     &task.Priority,
	})
}

func (p *Processor) handleTaskSuccess(task *model.Task, taskContext *amazonModel.TaskContext) {
	if task == nil {
		return
	}

	recordAmazonDailyListingCount(task, taskContext, p.GetMemoryManager(), p.GetLogger())

	p.updateTaskStatusSyncWithInput(taskstatus.UpdateInput{
		TaskID: task.ID,
		Status: model.TaskStatusPublished,
		Stage:  amazonTaskStagePublishCompleted,
	})
}

func (p *Processor) updateTaskStatusSyncWithInput(input taskstatus.UpdateInput) {
	statusService := taskstatus.NewService("amazon/processor", func() taskstatus.ImportTaskStatusClient {
		runtime := p.GetTaskStatusRuntime()
		if runtime == nil {
			return nil
		}
		return taskstatus.NewRuntimeTaskStatusAdapter(runtime)
	})

	if err := statusService.TransitionSyncWithInput(model.TaskStatusProcessing, input); err != nil {
		p.GetLogger().WithError(err).WithFields(logrus.Fields{
			"task_id": input.TaskID,
			"status":  input.Status.String(),
		}).Error("[Amazon] failed to update task status")
	}
}

func (p *Processor) pauseStoreForAuthentication(task *model.Task) {
	if task == nil {
		return
	}

	if memoryManager := p.GetMemoryManager(); memoryManager != nil {
		memoryManager.ShopPauseManager.PauseShopForDuration(
			task.TenantID,
			task.StoreID,
			"amazon authentication expired",
			10*time.Minute,
		)
	}

	storeClient := p.GetStoreAPI()
	if storeClient == nil {
		return
	}

	if success, err := storeClient.SetStorePauseStatus(task.StoreID, true, "auth_expired"); err != nil {
		p.GetLogger().WithError(err).Warnf("[Amazon] failed to set remote store pause status for auth expiry: store=%d", task.StoreID)
	} else if !success {
		p.GetLogger().Warnf("[Amazon] remote store pause status update returned unsuccessful for auth expiry: store=%d", task.StoreID)
	}
}

func recordAmazonDailyListingCount(task *model.Task, taskContext *amazonModel.TaskContext, memoryManager *state.MemoryManager, logger *logrus.Logger) {
	if memoryManager == nil || memoryManager.DailyCountManager == nil {
		return
	}
	if task == nil || taskContext == nil || taskContext.StoreInfo == nil {
		return
	}

	increment := calculateAmazonDailyListingIncrement(taskContext)
	if increment <= 0 {
		return
	}

	hasDailyLimit := taskContext.StoreInfo.DailyLimit != nil && *taskContext.StoreInfo.DailyLimit > 0
	if hasDailyLimit && taskContext.DailyQuotaReserved {
		if logger != nil {
			logger.WithFields(logrus.Fields{
				"tenant_id": task.TenantID,
				"store_id":  task.StoreID,
				"date":      taskContext.DailyQuotaDate,
				"increment": taskContext.DailyQuotaIncrement,
			}).Info("[Amazon] reuse reserved daily quota after publish success")
		}
		taskContext.ClearDailyQuotaReservation()
		return
	}

	currentDate := timex.NowDate()
	count := memoryManager.DailyCountManager.IncrementCount(
		task.TenantID,
		task.StoreID,
		currentDate,
		increment,
	)

	if logger != nil {
		logger.WithFields(logrus.Fields{
			"tenant_id": task.TenantID,
			"store_id":  task.StoreID,
			"date":      currentDate,
			"increment": increment,
			"count":     count,
		}).Info("[Amazon] recorded daily listing count after publish success")
	}
}

func calculateAmazonDailyListingIncrement(taskContext *amazonModel.TaskContext) int64 {
	if taskContext == nil || taskContext.StoreInfo == nil {
		return 1
	}

	limitType := taskContext.StoreInfo.DailyLimitType
	if limitType == "" {
		limitType = "SPU"
	}

	switch limitType {
	case "SKU", "SKC":
		if count, exists := taskContext.GetResult("variant_children_count"); exists {
			switch value := count.(type) {
			case int:
				if value > 0 {
					return int64(value)
				}
			case int64:
				if value > 0 {
					return value
				}
			case float64:
				if value > 0 {
					return int64(value)
				}
			}
		}
	}

	return 1
}

func isAmazonRetryableTaskError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.TrimSpace(strings.ToLower(err.Error()))
	if strings.HasPrefix(message, "nonretryable:") || strings.HasPrefix(message, "terminated:") {
		return false
	}
	return true
}

func shouldPauseAmazonTask(reasonCode string) bool {
	switch strings.TrimSpace(reasonCode) {
	case amazonTaskReasonAuthExpired, amazonTaskReasonDailyLimitReached:
		return true
	default:
		return false
	}
}

func parseTaskStatusMetadata(message string) (reasonCode string, stage string) {
	for _, token := range strings.Fields(strings.TrimSpace(message)) {
		if strings.HasPrefix(token, "[stage:") && strings.HasSuffix(token, "]") {
			stage = strings.TrimSuffix(strings.TrimPrefix(token, "[stage:"), "]")
			continue
		}
		if strings.HasPrefix(token, "[") && strings.HasSuffix(token, "]") {
			content := strings.TrimSuffix(strings.TrimPrefix(token, "["), "]")
			if content != "" && !strings.HasPrefix(content, "stage:") {
				reasonCode = content
			}
		}
	}
	return reasonCode, stage
}
