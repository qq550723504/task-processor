package product

import (
	"fmt"
	"time"

	"task-processor/internal/pkg/jsonx"
	models "task-processor/internal/temu/api/product"
)

func (h *SavePublishResultHandler) logSubmitResponseWithInput(input *SavePublishResultInput) error {
	responseJSON, err := jsonx.MarshalWithoutHTMLEscape(input.SubmitResponse)
	if err != nil {
		h.logger.Errorf("序列化响应数据失败: %v", err)
		return fmt.Errorf("序列化响应数据失败: %w", err)
	}

	h.logger.WithFields(input.SubmitResponseLogFields(string(responseJSON))).Info("TEMU产品提交响应数据")

	return nil
}

func (h *SavePublishResultHandler) createProductImportMappingWithInput(input *SavePublishResultInput) error {
	if !input.HasProduct() {
		h.logger.Warn("????????????????")
		return nil
	}

	createdCount := 0
	input.ForEachSKU(func(sku *models.Sku) {
		createReq := input.BuildImportMappingCreateReq(sku)
		if createReq == nil {
			h.logger.Warn("TEMU import mapping request build skipped due to invalid input")
			return
		}

		input.ApplyImportMappingMetadata(sku, createReq)

		_, err := h.mappingClient.CreateProductImportMapping(createReq)
		if err != nil {
			h.logger.Errorf("????????????: OutSkuSn=%s, Error=%v", sku.OutSkuSN, err)
			return
		}

		createdCount++
		h.logger.Debugf("????????????: OutSkuSn=%s", sku.OutSkuSN)
	})

	h.logger.Infof("????????????: ??=%d", createdCount)
	return nil
}

func (h *SavePublishResultHandler) recordDailyListingCountWithInput(input *SavePublishResultInput) {
	if h.memoryManager == nil {
		h.logger.Debug("??????????????????")
		return
	}
	dailyLimit, dailyLimitType, ok := input.DailyLimitConfig()
	if !ok {
		if _, storeID, scopeOK := input.TenantAndStoreIDs(); scopeOK {
			h.logger.Debugf("?? %d ?????????????????", storeID)
		} else {
			h.logger.Debug("?????????????????")
		}
		return
	}

	currentDate := time.Now().Format("2006-01-02")
	increment := input.DailyLimitIncrement(dailyLimitType)
	if increment <= 0 {
		h.logger.Warn("?????????????")
		return
	}

	tenantID, storeID, ok := input.TenantAndStoreIDs()
	if !ok {
		h.logger.Warn("TEMU发布结果缺少任务作用域，无法记录每日上架数量")
		return
	}

	count := h.memoryManager.DailyCountManager.IncrementCount(
		tenantID,
		storeID,
		currentDate,
		increment,
	)

	h.logger.Infof("?? %d ? %s ?????: %d (????: %d, ??: %s)",
		storeID, currentDate, count, increment, dailyLimitType)

	if count > int64(dailyLimit) {
		h.logger.Warnf("?? %d ? %s ?????(%d)?????(%d)??????",
			storeID, currentDate, count, dailyLimit)
		h.pauseShopUntilEndOfDayWithInput(input, input.DailyLimitExceededReason(count, dailyLimit))
	}
}

func (h *SavePublishResultHandler) pauseShopUntilEndOfDayWithInput(input *SavePublishResultInput, reason string) {
	if h.memoryManager == nil {
		return
	}

	tenantID, storeID, ok := input.TenantAndStoreIDs()
	if !ok {
		h.logger.Warn("TEMU发布结果缺少任务作用域，无法暂停店铺")
		return
	}

	h.memoryManager.ShopPauseManager.PauseShopUntilEndOfDay(
		tenantID,
		storeID,
		reason,
	)

	h.logger.Infof("已暂停店铺 %d:%d 上架到当日结束，原因: %s",
		tenantID, storeID, reason)
}
