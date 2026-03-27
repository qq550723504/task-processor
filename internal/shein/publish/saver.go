package publish

import (
	"task-processor/internal/core/logger"
	"task-processor/internal/model"

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
	baseLogger := s.logger
	if baseLogger == nil {
		baseLogger = logger.GetGlobalLogger("publish_saver")
	}
	notifier := NewTaskStatusNotifier("shein/publish_saver", baseLogger)
	notifier.Notify(input, model.TaskStatusDraft, "")
}
