package amazon

import (
	"strings"
	"time"

	"task-processor/internal/app/taskstatus"
	"task-processor/internal/model"

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

func (p *Processor) handleTaskSuccess(task *model.Task) {
	if task == nil {
		return
	}

	p.updateTaskStatusSyncWithInput(taskstatus.UpdateInput{
		TaskID: task.ID,
		Status: model.TaskStatusPublished,
		Stage:  amazonTaskStagePublishCompleted,
	})
}

func (p *Processor) updateTaskStatusSyncWithInput(input taskstatus.UpdateInput) {
	statusService := taskstatus.NewService("amazon/processor", func() taskstatus.ImportTaskStatusClient {
		managementClient := p.GetManagementClient()
		if managementClient == nil {
			return nil
		}
		return managementClient.GetImportTaskClient()
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

	managementClient := p.GetManagementClient()
	if managementClient == nil {
		return
	}
	storeClient := managementClient.GetStoreClient()
	if storeClient == nil {
		return
	}

	if success, err := storeClient.SetStorePauseStatus(task.StoreID, true, "auth_expired"); err != nil {
		p.GetLogger().WithError(err).Warnf("[Amazon] failed to set remote store pause status for auth expiry: store=%d", task.StoreID)
	} else if !success {
		p.GetLogger().Warnf("[Amazon] remote store pause status update returned unsuccessful for auth expiry: store=%d", task.StoreID)
	}
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
