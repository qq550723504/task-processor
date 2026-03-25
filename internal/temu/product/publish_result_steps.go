package product

import (
	"fmt"
	"time"

	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/pkg/ptr"
	pkgproduct "task-processor/internal/product"
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
		h.logger.Warn("产品数据不存在，无法创建映射关系")
		return nil
	}

	createdCount := 0
	for _, skc := range input.Product.SkcList {
		for _, sku := range skc.SkuList {
			createReq := &api.ProductImportMappingCreateReqDTO{
				ImportTaskId: input.Task.ID,
				TenantID:     input.Task.TenantID,
				StoreId:      input.Task.StoreID,
				Platform:     "TEMU",
				Region:       input.Task.Region,
				Sku:          &sku.OutSkuSN,
				ProductId:    "",
				Status:       ptr.Int16Ptr(1),
			}

			if input.AsinSkuMap != nil {
				if asin, exists := input.AsinSkuMap[sku.OutSkuSN]; exists {
					createReq.ProductId = asin
				}
			}

			if input.AmazonProduct != nil && input.AmazonProduct.ParentAsin != "" {
				createReq.ParentProductId = &input.AmazonProduct.ParentAsin
				if input.StoreInfo != nil && input.StoreInfo.PriceType != "" {
					costPrice := pkgproduct.GetProductPrice(input.AmazonProduct, input.StoreInfo.PriceType)
					if costPrice > 0 {
						createReq.CostPrice = &costPrice
					}
				}
			}

			if input.FilterRule != nil {
				createReq.FilterRuleId = &input.FilterRule.ID
				filterRuleRange := h.buildFilterRuleRange(input.FilterRule)
				if filterRuleRange != "" {
					createReq.FilterRuleRange = &filterRuleRange
				}
			}

			if input.ProfitRule != nil {
				createReq.ProfitRuleId = &input.ProfitRule.ID
				if input.ProfitRule.SalePriceMultiplier > 0 {
					salePriceMultiplierStr := fmt.Sprintf("%.4f", input.ProfitRule.SalePriceMultiplier)
					createReq.SalePriceMultiplier = &salePriceMultiplierStr
				}
				if input.ProfitRule.DiscountPriceMultiplier > 0 {
					discountPriceMultiplierStr := fmt.Sprintf("%.4f", input.ProfitRule.DiscountPriceMultiplier)
					createReq.DiscountPriceMultiplier = &discountPriceMultiplierStr
				}
			}

			_, err := h.mappingClient.CreateProductImportMapping(createReq)
			if err != nil {
				h.logger.Errorf("创建产品导入映射关系失败: OutSkuSn=%s, Error=%v", sku.OutSkuSN, err)
				continue
			}

			createdCount++
			h.logger.Debugf("成功创建产品导入映射关系: OutSkuSn=%s", sku.OutSkuSN)
		}
	}

	h.logger.Infof("产品导入映射关系创建完成: 成功=%d", createdCount)
	return nil
}

func (h *SavePublishResultHandler) recordDailyListingCountWithInput(input *SavePublishResultInput) {
	if h.memoryManager == nil {
		h.logger.Debug("内存管理器未初始化，跳过每日上架计数")
		return
	}
	if input.StoreInfo == nil {
		h.logger.Debug("店铺信息未初始化，跳过每日上架计数")
		return
	}
	if input.StoreInfo.DailyLimit == nil || *input.StoreInfo.DailyLimit <= 0 {
		h.logger.Debugf("店铺 %d 没有设置每日上架限额，跳过限额检查", input.Task.StoreID)
		return
	}

	dailyLimit := *input.StoreInfo.DailyLimit
	dailyLimitType := "SPU"
	if input.StoreInfo.DailyLimitType != "" {
		dailyLimitType = input.StoreInfo.DailyLimitType
	}

	currentDate := time.Now().Format("2006-01-02")
	increment := h.calculateIncrementFromInput(input, dailyLimitType)
	if increment <= 0 {
		h.logger.Warn("计算增量失败，跳过计数更新")
		return
	}

	count := h.memoryManager.DailyCountManager.IncrementCount(
		input.Task.TenantID,
		input.Task.StoreID,
		currentDate,
		increment,
	)

	h.logger.Infof("店铺 %d 在 %s 的上架计数: %d (本次增加: %d, 类型: %s)",
		input.Task.StoreID, currentDate, count, increment, dailyLimitType)

	if count > int64(dailyLimit) {
		h.logger.Warnf("店铺 %d 在 %s 的上架数量(%d)已超过限额(%d)，将暂停上架",
			input.Task.StoreID, currentDate, count, dailyLimit)
		h.pauseShopUntilEndOfDayWithInput(input, fmt.Sprintf("超过每日上架限额(%d/%d)", count, dailyLimit))
	}
}

func (h *SavePublishResultHandler) calculateIncrementFromInput(input *SavePublishResultInput, dailyLimitType string) int64 {
	if input.Product == nil {
		h.logger.Warn("TEMU产品数据为空，无法计算增量")
		return 0
	}

	switch dailyLimitType {
	case "SPU":
		return 1
	case "SKC":
		return int64(len(input.Product.SkcList))
	case "SKU":
		var skuCount int64
		for _, skc := range input.Product.SkcList {
			skuCount += int64(len(skc.SkuList))
		}
		return skuCount
	default:
		h.logger.Warnf("未知的限制类型: %s，默认按SPU计算", dailyLimitType)
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
