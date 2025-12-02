package handlers

import (
	"fmt"
	"strconv"
	"time"

	"task-processor/common/management/api"
	"task-processor/common/memory"
	"task-processor/common/pipeline"
	temutypes "task-processor/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// SavePublishResultHandler 保存发品成功后返回信息处理器（参考SHEIN实现）
type SavePublishResultHandler struct {
	mappingClient api.ProductImportMappingAPI
	memoryManager *memory.MemoryManager
	logger        *logrus.Entry
}

// NewSavePublishResultHandler 创建新的保存发品成功后返回信息处理器
func NewSavePublishResultHandler(mappingClient api.ProductImportMappingAPI, memoryManager *memory.MemoryManager) *SavePublishResultHandler {
	return &SavePublishResultHandler{
		mappingClient: mappingClient,
		memoryManager: memoryManager,
		logger:        logrus.WithField("handler", "SavePublishResultHandler"),
	}
}

// Name 返回处理器名称
func (h *SavePublishResultHandler) Name() string {
	return "保存发品成功后返回的信息"
}

// Handle 执行保存发品成功后返回信息处理
func (h *SavePublishResultHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始保存发品成功后的信息")

	// 检查是否有提交响应数据
	submitResponse, exists := ctx.GetData("submit_response")
	if !exists || submitResponse == nil {
		h.logger.Warn("TEMU提交响应数据为空，跳过保存")
		return nil
	}

	// 创建产品导入映射关系
	if err := h.createProductImportMapping(ctx); err != nil {
		h.logger.Warnf("创建产品导入映射关系失败: %v", err)
		// 不阻断流程，继续执行
	}

	// 记录每日上架成功数量并检查限额
	if err := h.recordDailyListingCount(ctx); err != nil {
		h.logger.Warnf("记录每日上架计数失败: %v", err)
		// 不阻断流程，继续执行
	}

	h.logger.Info("发品成功后返回信息保存完成")
	return nil
}

// createProductImportMapping 创建产品导入映射关系
func (h *SavePublishResultHandler) createProductImportMapping(ctx *pipeline.TaskContext) error {
	if ctx.Task == nil {
		return fmt.Errorf("任务信息未初始化")
	}

	// 获取提交响应数据
	submitResponseData, exists := ctx.GetData("submit_response")
	if !exists {
		h.logger.Warn("提交响应数据不存在")
		return nil
	}

	// 解析任务ID
	taskID, err := strconv.ParseInt(ctx.Task.ID, 10, 64)
	if err != nil {
		return fmt.Errorf("解析任务ID失败: %w", err)
	}

	// 获取产品数据
	productData, exists := ctx.GetData("product_data")
	if !exists || productData == nil {
		h.logger.Warn("产品数据不存在，无法创建映射关系")
		return nil
	}

	// 类型断言为 TEMU Product
	temuProduct, ok := productData.(*temutypes.Product)
	if !ok {
		h.logger.Warn("产品数据类型转换失败")
		return nil
	}

	createdCount := 0

	// 遍历SKC和SKU列表创建映射关系
	if len(temuProduct.SkcList) > 0 {
		for _, skc := range temuProduct.SkcList {
			for _, sku := range skc.SkuList {
				createReq := &api.ProductImportMappingCreateReqDTO{
					TenantID:     ctx.Task.TenantID,
					ImportTaskId: taskID,
					StoreId:      ctx.Task.StoreID,
					Platform:     "TEMU",
					Region:       ctx.Task.Region,
					Status:       int16Ptr(2), // 2表示已上架
				}

				// 设置SKU信息
				if sku.OutSkuSN != "" {
					createReq.Sku = &sku.OutSkuSN
				}

				// 从提交响应中获取平台产品ID
				// 注意：TEMU的响应结构可能不同，这里先简单处理
				if submitResp, ok := submitResponseData.(map[string]interface{}); ok {
					if productID, exists := submitResp["product_id"]; exists {
						if pid, ok := productID.(string); ok && pid != "" {
							createReq.PlatformProductId = &pid
						}
					}
				}

				// 从AsinSkuMap中查找对应的ASIN
				if asinSkuMapData, exists := ctx.GetData("asin_sku_map"); exists {
					if asinSkuMap, ok := asinSkuMapData.(map[string]string); ok {
						// 映射关系是 SKU -> ASIN
						if asin, found := asinSkuMap[sku.OutSkuSN]; found {
							createReq.ProductId = asin
						}
					}
				}

				// 设置成本价（从SKU的供应商价格获取）
				if sku.SupplierPrice > 0 {
					costPrice := float64(sku.SupplierPrice) / 100.0 // 转换为美元
					createReq.CostPrice = &costPrice
				}

				// 设置父产品ASIN
				if ctx.AmazonProduct != nil && ctx.AmazonProduct.ParentAsin != "" {
					createReq.ParentProductId = &ctx.AmazonProduct.ParentAsin
				}

				// 设置平台父产品ID（使用外部商品编号）
				if temuProduct.GoodsBasic.OutGoodsSN != "" {
					createReq.PlatformParentProductId = &temuProduct.GoodsBasic.OutGoodsSN
				}

				// 设置筛选规则信息
				if filterRuleData, exists := ctx.GetData("filter_rule"); exists {
					if filterRule, ok := filterRuleData.(*api.FilterRuleRespDTO); ok {
						createReq.FilterRuleId = &filterRule.ID
						if filterRule.PriceMin != nil && filterRule.PriceMax != nil {
							filterRuleRange := fmt.Sprintf("%.2f-%.2f", *filterRule.PriceMin, *filterRule.PriceMax)
							createReq.FilterRuleRange = &filterRuleRange
						} else if filterRule.PriceMin != nil {
							filterRuleRange := fmt.Sprintf("%.2f-", *filterRule.PriceMin)
							createReq.FilterRuleRange = &filterRuleRange
						} else if filterRule.PriceMax != nil {
							filterRuleRange := fmt.Sprintf("-%.2f", *filterRule.PriceMax)
							createReq.FilterRuleRange = &filterRuleRange
						}
					}
				}

				// 设置利润规则信息
				if profitRuleData, exists := ctx.GetData("profit_rule"); exists {
					if profitRule, ok := profitRuleData.(*api.ProfitRuleRespDTO); ok {
						createReq.ProfitRuleId = &profitRule.ID
						salePriceMultiplier := fmt.Sprintf("%.2f", profitRule.SalePriceMultiplier)
						createReq.SalePriceMultiplier = &salePriceMultiplier

						if profitRule.DiscountPriceMultiplier > 0 {
							discountPriceMultiplier := fmt.Sprintf("%.2f", profitRule.DiscountPriceMultiplier)
							createReq.DiscountPriceMultiplier = &discountPriceMultiplier
						}
					}
				}

				// 记录请求数据用于调试
				h.logger.Infof("准备创建产品导入映射关系: SKU=%s, ASIN=%s, PlatformProductId=%v, ParentProductId=%v, PlatformParentProductId=%v",
					sku.OutSkuSN,
					createReq.ProductId,
					createReq.PlatformProductId,
					createReq.ParentProductId,
					createReq.PlatformParentProductId)

				// 调用API创建产品导入映射关系
				id, err := h.mappingClient.CreateProductImportMapping(createReq)
				if err != nil {
					h.logger.Errorf("创建产品导入映射关系失败: %v, 请求数据: TenantID=%d, ImportTaskId=%d, StoreId=%d, Platform=%s, Region=%s, SKU=%s, ProductId=%s",
						err,
						createReq.TenantID,
						createReq.ImportTaskId,
						createReq.StoreId,
						createReq.Platform,
						createReq.Region,
						getStringValue(createReq.Sku),
						createReq.ProductId)
					continue
				}

				h.logger.Infof("成功创建产品导入映射关系，ID: %d, SKU: %s, ASIN: %s",
					id, sku.OutSkuSN, createReq.ProductId)
				createdCount++
			}
		}
	}

	h.logger.Infof("成功创建 %d 个产品导入映射关系", createdCount)
	return nil
}

// int16Ptr 返回int16指针
func int16Ptr(i int16) *int16 {
	return &i
}

// getStringValue 安全获取字符串指针的值
func getStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// recordDailyListingCount 记录每日上架成功数量并检查限额（参考SHEIN实现）
func (h *SavePublishResultHandler) recordDailyListingCount(ctx *pipeline.TaskContext) error {
	// 检查必要的上下文信息
	if h.memoryManager == nil {
		h.logger.Debug("内存管理器未初始化，跳过每日上架计数")
		return nil
	}

	if ctx.Task == nil {
		h.logger.Debug("任务信息未初始化，跳过每日上架计数")
		return nil
	}

	if ctx.StoreInfo == nil {
		h.logger.Debug("店铺信息未初始化，跳过每日上架计数")
		return nil
	}

	// 检查店铺是否有每日上架限额
	if ctx.StoreInfo.DailyLimit == nil || *ctx.StoreInfo.DailyLimit <= 0 {
		h.logger.Debugf("店铺 %d 没有设置每日上架限额，跳过限额检查", ctx.StoreInfo.ID)
		return nil
	}

	dailyLimit := *ctx.StoreInfo.DailyLimit
	h.logger.Debugf("店铺 %d 的每日上架限额为: %d，限制类型: %s", ctx.StoreInfo.ID, dailyLimit, ctx.StoreInfo.DailyLimitType)

	// 获取当前日期（格式：YYYY-MM-DD）
	currentDate := time.Now().Format("2006-01-02")

	// 根据店铺配置的限制类型计算增加的数量
	increment := h.calculateIncrement(ctx)
	if increment <= 0 {
		h.logger.Warnf("计算增量失败，跳过计数更新")
		return nil
	}

	// 增加每日上架计数
	count := h.memoryManager.DailyCountManager.IncrementCount(
		ctx.Task.TenantID,
		ctx.Task.StoreID,
		currentDate,
		increment,
	)

	h.logger.Infof("店铺 %d 在 %s 的上架计数: %d (本次增加: %d, 类型: %s)",
		ctx.StoreInfo.ID, currentDate, count, increment, ctx.StoreInfo.DailyLimitType)

	// 检查是否超过限额
	if count > int64(dailyLimit) {
		h.logger.Warnf("店铺 %d 在 %s 的上架数量(%d)已超过限额(%d)，将暂停上架", ctx.StoreInfo.ID, currentDate, count, dailyLimit)

		// 暂停店铺上架到当日结束
		if err := h.pauseShopUntilEndOfDay(
			ctx,
			fmt.Sprintf("超过每日上架限额(%d/%d)", count, dailyLimit),
		); err != nil {
			h.logger.Errorf("暂停店铺上架失败: %v", err)
		}

		h.logger.Infof("已暂停店铺 %d 上架到当日结束，因为已超过每日限额 %d", ctx.StoreInfo.ID, dailyLimit)
	} else {
		h.logger.Infof("店铺 %d 在 %s 的上架数量(%d)未超过限额(%d)", ctx.StoreInfo.ID, currentDate, count, dailyLimit)
	}

	return nil
}

// calculateIncrement 根据店铺配置的限制类型计算增量
func (h *SavePublishResultHandler) calculateIncrement(ctx *pipeline.TaskContext) int64 {
	// 检查TEMU产品数据是否存在
	if ctx.TemuProduct == nil {
		h.logger.Warn("TEMU产品数据为空，无法计算增量")
		return 0
	}

	switch ctx.StoreInfo.DailyLimitType {
	case "SPU":
		// SPU级别：每个产品算1个
		return 1
	case "SKC":
		// SKC级别：按SKC数量计算
		skcCount := int64(len(ctx.TemuProduct.SkcList))
		h.logger.Debugf("SKC计数: %d", skcCount)
		return skcCount
	case "SKU":
		// SKU级别：按所有SKU数量计算
		var skuCount int64
		for _, skc := range ctx.TemuProduct.SkcList {
			skuCount += int64(len(skc.SkuList))
		}
		h.logger.Debugf("SKU计数: %d", skuCount)
		return skuCount
	default:
		// 默认按SPU计算
		h.logger.Warnf("未知的限制类型: %s，默认按SPU计算", ctx.StoreInfo.DailyLimitType)
		return 1
	}
}

// pauseShopUntilEndOfDay 暂停店铺到当日结束
func (h *SavePublishResultHandler) pauseShopUntilEndOfDay(ctx *pipeline.TaskContext, reason string) error {
	// 暂停店铺到当日结束（23:59:59）
	h.memoryManager.ShopPauseManager.PauseShopUntilEndOfDay(
		ctx.Task.TenantID,
		ctx.Task.StoreID,
		reason,
	)

	h.logger.Infof("已暂停店铺 %d:%d 上架到当日结束，原因: %s", ctx.Task.TenantID, ctx.Task.StoreID, reason)

	return nil
}
