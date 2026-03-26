package publish

import (
	"time"

	"task-processor/internal/core/logger"
	management_api "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	"task-processor/internal/pkg/recovery"
	"task-processor/internal/pkg/timex"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/validation"

	"github.com/sirupsen/logrus"
)

// SavePublishResultHandler persists post-publish side effects.
type SavePublishResultHandler struct {
	logger *logrus.Entry
}

// NewSavePublishResultHandler creates a result handler.
func NewSavePublishResultHandler() *SavePublishResultHandler {
	return &SavePublishResultHandler{
		logger: logger.GetGlobalLogger("save_publish_result"),
	}
}

// Name returns the handler name.
func (h *SavePublishResultHandler) Name() string {
	return "保存发品成功后返回的信息"
}

// Handle persists mapping records, daily counters, and task status updates.
func (h *SavePublishResultHandler) Handle(ctx *shein.TaskContext) error {
	if ctx.ProductData == nil {
		return shein.NewNonRetryableError("产品数据未获取，请先执行获取产品数据步骤", nil)
	}

	input, err := buildPublishResultInput(ctx)
	if err != nil {
		return shein.NewNonRetryableError("构建发布结果输入失败", err)
	}

	if err := h.createProductImportMapping(input); err != nil {
		h.logger.Warnf("创建产品导入映射关系失败: %v", err)
	}

	h.recordDailyListingCount(input)
	updateTaskStatusToPublished(input)

	h.logger.Info("发品成功后返回信息保存完成")
	return nil
}

func (h *SavePublishResultHandler) createProductImportMapping(input *PublishResultInput) error {
	if input.ManagementClientMgr == nil {
		return shein.NewNonRetryableError("管理客户端管理器未初始化", nil)
	}
	if input.Task == nil {
		return shein.NewNonRetryableError("任务信息未初始化", nil)
	}

	mappingClient := input.ManagementClientMgr.GetProductImportMappingClient()
	if mappingClient == nil {
		return shein.NewNonRetryableError("产品导入映射客户端未初始化", nil)
	}

	if input.SheinResponse == nil || len(input.SheinResponse.Info.SKCList) == 0 {
		return nil
	}

	processed := make(map[string]bool)
	createdCount := 0

	for _, skc := range input.SheinResponse.Info.SKCList {
		for _, sku := range skc.SKUList {
			if processed[sku.SupplierSKU] {
				h.logger.Debugf("SKU %s already processed, skip duplicate mapping", sku.SupplierSKU)
				continue
			}
			processed[sku.SupplierSKU] = true

			asin := resolveAsinForPublishedSKU(input, sku.SupplierSKU)
			if asin == "" {
				h.logger.Errorf("SKU %s missing matching ASIN and task product id, skip", sku.SupplierSKU)
				continue
			}

			createReq := buildMappingReq(input.MappingInput, asin, sku.SupplierSKU, model.TaskStatusPublished)
			createReq.PlatformProductId = &sku.SKUCode

			existing, err := mappingClient.GetProductImportMappingByTaskAndSku(input.Task.ID, sku.SupplierSKU)
			if err != nil {
				h.logger.Warnf("query existing mapping failed (SKU: %s): %v", sku.SupplierSKU, err)
			}

			var id int64
			if existing != nil && existing.ID > 0 {
				createReq.ID = &existing.ID
				if err := mappingClient.UpdateProductImportMapping(createReq); err != nil {
					h.logger.Errorf("update product mapping failed (SKU: %s): %v", sku.SupplierSKU, err)
					continue
				}
				id = existing.ID
				h.logger.Infof("updated product mapping - ID: %d, SKU: %s, PlatformSKU: %s", id, sku.SupplierSKU, sku.SKUCode)
			} else {
				id, err = mappingClient.CreateProductImportMapping(createReq)
				if err != nil {
					h.logger.Errorf("create product mapping failed (SKU: %s): %v", sku.SupplierSKU, err)
					continue
				}
				h.logger.Infof("created product mapping - ID: %d, SKU: %s, PlatformSKU: %s", id, sku.SupplierSKU, sku.SKUCode)
			}
			createdCount++
		}
	}

	h.logger.Infof("created %d product import mappings", createdCount)
	return nil
}

func resolveAsinForPublishedSKU(input *PublishResultInput, supplierSKU string) string {
	for asin, sku := range input.AsinSkuMap {
		if sku == supplierSKU {
			return asin
		}
	}
	if input.Task != nil {
		return input.Task.ProductID
	}
	return ""
}

func (h *SavePublishResultHandler) recordDailyListingCount(input *PublishResultInput) {
	if input.MemoryManager == nil {
		h.logger.Warn("memory manager is nil, skip daily listing count")
		return
	}
	if input.Task == nil {
		h.logger.Warn("task is nil, skip daily listing count")
		return
	}
	if input.StoreInfo == nil {
		h.logger.Warn("store info is nil, skip daily listing count")
		return
	}
	if input.StoreInfo.DailyLimit == nil || *input.StoreInfo.DailyLimit <= 0 {
		h.logger.Debugf("store %d has no daily limit, skip limit check", input.StoreInfo.ID)
		return
	}

	dailyLimit := *input.StoreInfo.DailyLimit
	currentDate := timex.NowDate()
	increment := h.calculateIncrement(input)
	if increment <= 0 {
		h.logger.Warn("calculated listing increment is 0, skip count update")
		return
	}

	count := input.MemoryManager.DailyCountManager.IncrementCount(
		input.Task.TenantID,
		input.Task.StoreID,
		currentDate,
		increment,
	)

	h.logger.Infof("store %d listing count on %s is %d (increment: %d, type: %s)",
		input.StoreInfo.ID, currentDate, count, increment, input.StoreInfo.DailyLimitType)

	if count > int64(dailyLimit) {
		h.logger.Warnf("store %d exceeded daily limit %d with count %d, pause listing", input.StoreInfo.ID, dailyLimit, count)
		h.pauseShopWithCacheCleanup(input, "超过每日上架限额", 24*time.Hour)
		h.logger.Infof("store %d paused for 24 hours after exceeding daily limit %d", input.StoreInfo.ID, dailyLimit)
		return
	}

	h.logger.Infof("store %d remains under daily limit %d on %s", input.StoreInfo.ID, dailyLimit, currentDate)
}

func (h *SavePublishResultHandler) calculateIncrement(input *PublishResultInput) int64 {
	if input.SheinResponse == nil {
		h.logger.Warn("shein response is nil, cannot calculate listing increment")
		return 0
	}
	ctx := &shein.TaskContext{}
	ctx.SetSheinResponse(input.SheinResponse)
	return validation.EstimateListingIncrement(ctx)
}

func (h *SavePublishResultHandler) pauseShopWithCacheCleanup(input *PublishResultInput, reason string, duration time.Duration) {
	if input.MemoryManager == nil || input.Task == nil {
		return
	}

	h.logger.Infof("clearing cache for store %d:%d before pause", input.Task.TenantID, input.Task.StoreID)
	input.MemoryManager.ShopPauseManager.PauseShop(
		input.Task.TenantID,
		input.Task.StoreID,
		reason,
		duration,
	)
}

func updateTaskStatusToPublished(input *PublishResultInput) {
	log := logger.GetGlobalLogger("publish_result")

	statusInput := &TaskStatusUpdateInput{
		Task:                input.Task,
		ManagementClientMgr: input.ManagementClientMgr,
	}
	if statusInput.ManagementClientMgr == nil {
		log.Warn("management client manager is nil, skip published status update")
		return
	}
	if statusInput.Task == nil {
		log.Warn("task is nil, skip published status update")
		return
	}

	importTaskClient := statusInput.ManagementClientMgr.GetImportTaskClient()
	if importTaskClient == nil {
		log.Warn("import task client is nil, skip published status update")
		return
	}

	req := &management_api.ProductImportTaskUpdateReqDTO{
		ID:     statusInput.Task.ID,
		Status: model.TaskStatusPublished.Int16(),
	}
	taskID := statusInput.Task.ID

	go func() {
		defer recovery.Recover("update published task status", log.WithField("task_id", taskID))

		if err := importTaskClient.UpdateTaskStatus(req); err != nil {
			log.Errorf("update published task status failed (TaskID: %d): %v", taskID, err)
		} else {
			log.Infof("task status updated to published (TaskID: %d)", taskID)
		}
	}()
}
