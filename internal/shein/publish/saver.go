package publish

import (
	"task-processor/internal/core/logger"
	management_api "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	"task-processor/internal/pkg/recovery"

	"github.com/sirupsen/logrus"
)

// PublishProductSaver persists publish-side state updates.
type PublishProductSaver struct {
	logger *logrus.Entry
}

// NewPublishProductSaver creates a saver for publish results.
func NewPublishProductSaver() *PublishProductSaver {
	return &PublishProductSaver{
		logger: logger.GetGlobalLogger("publish_saver"),
	}
}

// SavePublishResult applies publish response data to local task state.
func (s *PublishProductSaver) SavePublishResult(input *SavePublishStateInput) error {
	if input.SetSupplierSkuMapFn == nil {
		return nil
	}

	if input.SheinResponse.Info.SPUName != "" {
		input.ProductData.SPUName = input.SheinResponse.Info.SPUName
	}

	for _, skc := range input.SheinResponse.Info.SKCList {
		for _, sku := range skc.SKUList {
			input.SetSupplierSkuMapFn(sku.SKUCode, sku.SupplierSKU)
		}
	}

	return nil
}

// UpdateTaskStatusToDraft marks the task as saved to draft asynchronously.
func (s *PublishProductSaver) UpdateTaskStatusToDraft(input *TaskStatusUpdateInput) {
	if input.ManagementClientMgr == nil {
		s.logger.Warn("management client manager is nil, skip draft status update")
		return
	}

	if input.Task == nil {
		s.logger.Warn("task is nil, skip draft status update")
		return
	}

	importTaskClient := input.ManagementClientMgr.GetImportTaskClient()
	if importTaskClient == nil {
		s.logger.Warn("import task client is nil, skip draft status update")
		return
	}

	taskID := input.Task.ID
	req := &management_api.ProductImportTaskUpdateReqDTO{
		ID:     taskID,
		Status: model.TaskStatusDraft.Int16(),
	}

	baseLogger := s.logger
	if baseLogger == nil {
		baseLogger = logger.GetGlobalLogger("publish_saver")
	}
	taskLogger := baseLogger.WithField("task_id", taskID)

	go func() {
		defer recovery.RecoverWithStack("update draft task status", taskLogger)

		if err := importTaskClient.UpdateTaskStatus(req); err != nil {
			taskLogger.Errorf("update draft task status failed (TaskID: %d): %v", taskID, err)
		} else {
			taskLogger.Infof("task status updated to draft (TaskID: %d)", taskID)
		}
	}()
}
