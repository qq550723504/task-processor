package product

import (
	"fmt"
	"time"

	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/pkg/jsonx"
	models "task-processor/internal/temu/api/product"
)

func (h *SavePublishResultHandler) logSubmitResponseWithInput(input *SavePublishResultInput) error {
	responseJSON, err := jsonx.MarshalWithoutHTMLEscape(input.SubmitResponse)
	if err != nil {
		h.logger.Errorf("序列化响应数据失败: %v", err)
		return fmt.Errorf("序列化响应数据失败: %w", err)
	}

	h.logger.WithFields(map[string]any{
		"task_id":    input.Task.ID,
		"tenant_id":  input.Task.TenantID,
		"store_id":   input.Task.StoreID,
		"platform":   input.Task.Platform,
		"product_id": input.Task.ProductID,
		"response":   string(responseJSON),
	}).Info("TEMU产品提交响应数据")

	return nil
}

func (h *SavePublishResultHandler) createProductImportMappingWithInput(input *SavePublishResultInput) error {
	if input.Product == nil {
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

		h.applyImportMappingMetadata(input, sku, createReq)

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

func (h *SavePublishResultHandler) applyImportMappingMetadata(
	input *SavePublishResultInput,
	sku *models.Sku,
	createReq *api.ProductImportMappingCreateReqDTO,
) {
	if productID, ok := input.ProductIDForSKU(sku); ok {
		createReq.ProductId = productID
	}

	if parentProductID, ok := input.ParentProductID(); ok {
		createReq.ParentProductId = &parentProductID
		if costPrice, ok := input.CostPrice(); ok {
			createReq.CostPrice = &costPrice
		}
	}

	if filterRuleID, ok := input.FilterRuleID(); ok {
		createReq.FilterRuleId = &filterRuleID
		if filterRuleRange, ok := input.FilterRuleRange(); ok {
			createReq.FilterRuleRange = &filterRuleRange
		}
	}

	if profitRuleID, ok := input.ProfitRuleID(); ok {
		createReq.ProfitRuleId = &profitRuleID
		if salePriceMultiplier, ok := input.SalePriceMultiplier(); ok {
			createReq.SalePriceMultiplier = &salePriceMultiplier
		}
		if discountPriceMultiplier, ok := input.DiscountPriceMultiplier(); ok {
			createReq.DiscountPriceMultiplier = &discountPriceMultiplier
		}
	}
}

func (h *SavePublishResultHandler) recordDailyListingCountWithInput(input *SavePublishResultInput) {
	if h.memoryManager == nil {
		h.logger.Debug("??????????????????")
		return
	}
	if input.StoreInfo == nil {
		h.logger.Debug("?????????????????")
		return
	}

	dailyLimit, dailyLimitType, ok := input.DailyLimitConfig()
	if !ok {
		h.logger.Debugf("?? %d ?????????????????", input.Task.StoreID)
		return
	}

	currentDate := time.Now().Format("2006-01-02")
	increment := h.calculateIncrementFromInput(input, dailyLimitType)
	if increment <= 0 {
		h.logger.Warn("?????????????")
		return
	}

	count := h.memoryManager.DailyCountManager.IncrementCount(
		input.Task.TenantID,
		input.Task.StoreID,
		currentDate,
		increment,
	)

	h.logger.Infof("?? %d ? %s ?????: %d (????: %d, ??: %s)",
		input.Task.StoreID, currentDate, count, increment, dailyLimitType)

	if count > int64(dailyLimit) {
		h.logger.Warnf("?? %d ? %s ?????(%d)?????(%d)??????",
			input.Task.StoreID, currentDate, count, dailyLimit)
		h.pauseShopUntilEndOfDayWithInput(input, input.DailyLimitExceededReason(count, dailyLimit))
	}
}

func (h *SavePublishResultHandler) calculateIncrementFromInput(input *SavePublishResultInput, dailyLimitType string) int64 {
	if input.Product == nil {
		h.logger.Warn("TEMU?????????????")
		return 0
	}

	switch dailyLimitType {
	case "SPU":
		return 1
	case "SKC":
		return int64(input.SKCCount())
	case "SKU":
		return int64(input.SKUCount())
	default:
		h.logger.Warnf("???????: %s????SPU??", dailyLimitType)
		return 1
	}
}

func (h *SavePublishResultHandler) pauseShopUntilEndOfDayWithInput(input *SavePublishResultInput, reason string) {
	if h.memoryManager == nil {
		return
	}

	h.memoryManager.ShopPauseManager.PauseShopUntilEndOfDay(
		input.Task.TenantID,
		input.Task.StoreID,
		reason,
	)

	h.logger.Infof("已暂停店铺 %d:%d 上架到当日结束，原因: %s",
		input.Task.TenantID, input.Task.StoreID, reason)
}
