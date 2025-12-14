package modules

import (
	"fmt"
	"strconv"
	"time"

	management_api "task-processor/common/management/api"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

// SavePublishResultHandler 保存发品成功后返回信息处理器
type SavePublishResultHandler struct {
}

// NewSavePublishResultHandler 创建新的保存发品成功后返回信息处理器
func NewSavePublishResultHandler() *SavePublishResultHandler {
	return &SavePublishResultHandler{}
}

// Name 返回处理器名称
func (h *SavePublishResultHandler) Name() string {
	return "保存发品成功后返回的信息"
}

// Handle 执行保存发品成功后返回信息处理
func (h *SavePublishResultHandler) Handle(ctx *TaskContext) error {
	// 检查是否已获取产品数据
	if ctx.ProductData == nil {
		// 这是一个程序逻辑错误，不应该发生，不可重试
		return NewNonRetryableError("产品数据未获取，请先执行获取产品数据步骤", nil)
	}

	// 创建产品导入映射关系
	if err := h.createProductImportMapping(ctx); err != nil {
		// 创建映射关系失败可能是网络或系统问题，可重试
		logrus.Warnf("创建产品导入映射关系失败%v", err)
	}

	// 记录每日上架成功数量并检查限额
	if err := h.recordDailyListingCount(ctx); err != nil {
		// 记录计数失败可能是网络或系统问题，可重试
		logrus.Warnf("记录每日上架计数失败%v", err)
	}

	// 更新任务状态为已上架
	if err := h.updateTaskStatusToPublished(ctx); err != nil {
		logrus.Warnf("更新任务状态为已上架失败: %v", err)
	}

	logrus.Println("发品成功后返回信息保存完成")

	return nil
}

// createProductImportMapping 创建产品导入映射关系
func (h *SavePublishResultHandler) createProductImportMapping(ctx *TaskContext) error {
	// 检查必要的上下文信息
	if ctx.ManagementClientMgr == nil {
		// 这是一个程序逻辑错误，不应该发生，不可重试
		return NewNonRetryableError("管理客户端管理器未初始化", nil)
	}

	if ctx.Task == nil {
		// 这是一个程序逻辑错误，不应该发生，不可重试
		return NewNonRetryableError("任务信息未初始化", nil)
	}

	// 获取产品导入映射客户端
	mappingClient := ctx.ManagementClientMgr.GetProductImportMappingClient()
	if mappingClient == nil {
		// 这是一个程序逻辑错误，不应该发生，不可重试
		return NewNonRetryableError("产品导入映射客户端未初始化", nil)
	}

	// 为每个SKU创建产品导入映射关系
	createdCount := 0

	// 遍历SheinResponse中的SKC和SKU信息来创建映射关系
	// 使用map去重，避免同一个SupplierSKU创建多次映射
	processedSkus := make(map[string]bool)

	if ctx.SheinResponse != nil && len(ctx.SheinResponse.Info.SKCList) > 0 {
		for _, skc := range ctx.SheinResponse.Info.SKCList {
			for _, sku := range skc.SKUList {
				// 检查是否已处理过该SupplierSKU
				if processedSkus[sku.SupplierSKU] {
					logrus.Debugf("SKU %s 已处理，跳过重复创建", sku.SupplierSKU)
					continue
				}
				processedSkus[sku.SupplierSKU] = true

				// 构建ProductImportMappingCreateReqDTO结构体
				taskID, err := strconv.ParseInt(ctx.Task.ID, 10, 64)
				if err != nil {
					logrus.Errorf("转换任务ID失败: %v", err)
					continue
				}
				createReq := &management_api.ProductImportMappingCreateReqDTO{
					TenantID:          ctx.Task.TenantID,
					ImportTaskId:      taskID,
					StoreId:           ctx.Task.StoreID,
					Platform:          "SHEIN",
					Region:            ctx.Task.Region,
					Sku:               &sku.SupplierSKU,
					PlatformProductId: &sku.SKUCode,
					Status:            int16Ptr(1), // 1表示成功
				}

				// 从AsinSkuMap中查找ASIN（需要反向查找，因为map是ASIN->SKU的映射）
				foundAsin := false
				if ctx.AsinSkuMap != nil {
					for asin, supplierSku := range ctx.AsinSkuMap {
						if supplierSku == sku.SupplierSKU {
							createReq.ProductId = asin
							foundAsin = true
							break
						}
					}
				}

				// 如果在AsinSkuMap中没找到，使用任务的ProductID作为备选
				if !foundAsin || createReq.ProductId == "" {
					if ctx.Task != nil && ctx.Task.ProductID != "" {
						createReq.ProductId = ctx.Task.ProductID
						logrus.Warnf("SKU %s 在AsinSkuMap中未找到对应ASIN，使用任务ProductID: %s",
							sku.SupplierSKU, ctx.Task.ProductID)
					} else {
						logrus.Errorf("SKU %s 未找到对应的ASIN且任务ProductID为空，跳过创建映射关系", sku.SupplierSKU)
						continue
					}
				}

				variant := GetVariantByAsinFromVariants(ctx.Variants, createReq.ProductId)
				costPrice := GetProductPrice(variant, ctx.StoreInfo.PriceType)
				createReq.CostPrice = &costPrice

				if ctx.AmazonProduct.ParentAsin != "" {
					createReq.ParentProductId = &ctx.AmazonProduct.ParentAsin
				}

				if ctx.ProductData != nil && ctx.ProductData.SPUName != "" {
					createReq.PlatformParentProductId = &ctx.ProductData.SPUName
				}

				// 如果有筛选规则，设置筛选规则相关字段
				if ctx.FilterRule != nil {
					createReq.FilterRuleId = &ctx.FilterRule.ID
					// 修复：正确处理指针类型并生成价格范围字符串
					if ctx.FilterRule.PriceMin != nil && ctx.FilterRule.PriceMax != nil {
						filterRuleRange := fmt.Sprintf("%.2f-%.2f", *ctx.FilterRule.PriceMin, *ctx.FilterRule.PriceMax)
						createReq.FilterRuleRange = &filterRuleRange
					} else if ctx.FilterRule.PriceMin != nil {
						filterRuleRange := fmt.Sprintf("%.2f-", *ctx.FilterRule.PriceMin)
						createReq.FilterRuleRange = &filterRuleRange
					} else if ctx.FilterRule.PriceMax != nil {
						filterRuleRange := fmt.Sprintf("-%.2f", *ctx.FilterRule.PriceMax)
						createReq.FilterRuleRange = &filterRuleRange
					}
				}

				// 如果有利润规则，设置利润规则相关字段
				if ctx.ProfitRule != nil {
					createReq.ProfitRuleId = &ctx.ProfitRule.ID
					// 设置售价倍数和折扣价倍数
					salePriceMultiplier := fmt.Sprintf("%.2f", ctx.ProfitRule.SalePriceMultiplier)
					createReq.SalePriceMultiplier = &salePriceMultiplier

					if ctx.ProfitRule.DiscountPriceMultiplier > 0 {
						discountPriceMultiplier := fmt.Sprintf("%.2f", ctx.ProfitRule.DiscountPriceMultiplier)
						createReq.DiscountPriceMultiplier = &discountPriceMultiplier
					}
				}

				// 检查是否已存在映射关系（避免重试时重复插入）
				existingMapping, err := mappingClient.GetProductImportMappingByTaskAndSku(
					taskID,
					sku.SupplierSKU,
				)

				if err != nil {
					logrus.Warnf("查询已存在的映射关系失败 (SKU: %s): %v，尝试创建新记录", sku.SupplierSKU, err)
				}

				var id int64
				if existingMapping != nil && existingMapping.ID > 0 {
					// 已存在记录，更新而不是插入
					logrus.Infof("检测到已存在的映射关系 (ID: %d, SKU: %s)，执行更新操作",
						existingMapping.ID, sku.SupplierSKU)

					createReq.ID = &existingMapping.ID
					if err := mappingClient.UpdateProductImportMapping(createReq); err != nil {
						logrus.Errorf("更新产品导入映射关系失败 (SKU: %s): %v", sku.SupplierSKU, err)
						continue
					}
					id = existingMapping.ID
					logrus.Infof("✅ 成功更新产品映射关系 - ID: %d, SKU: %s, PlatformSKU: %s",
						id, sku.SupplierSKU, sku.SKUCode)
				} else {
					// 不存在记录，创建新记录
					id, err = mappingClient.CreateProductImportMapping(createReq)
					if err != nil {
						logrus.Errorf("创建产品导入映射关系失败 (SKU: %s): %v", sku.SupplierSKU, err)
						continue
					}
					logrus.Infof("✅ 成功创建产品映射关系 - ID: %d, SKU: %s, PlatformSKU: %s",
						id, sku.SupplierSKU, sku.SKUCode)
				}

				createdCount++
			}
		}
	}

	logrus.Printf("成功创建 %d 个产品导入映射关系", createdCount)
	return nil
}

// recordDailyListingCount 记录每日上架成功数量并检查限额
func (h *SavePublishResultHandler) recordDailyListingCount(ctx *TaskContext) error {
	// 检查必要的上下文信息
	if ctx.MemoryManager == nil {
		logrus.Warn("内存管理器未初始化，跳过每日上架计数")
		return nil
	}

	if ctx.Task == nil {
		logrus.Warn("任务信息未初始化，跳过每日上架计数")
		return nil
	}

	if ctx.StoreInfo == nil {
		logrus.Warn("店铺信息未初始化，跳过每日上架计数")
		return nil
	}

	// 检查店铺是否有每日上架限额
	if ctx.StoreInfo.DailyLimit == nil || *ctx.StoreInfo.DailyLimit <= 0 {
		logrus.Debugf("店铺 %d 没有设置每日上架限额，跳过限额检查", ctx.StoreInfo.ID)
		return nil
	}

	dailyLimit := *ctx.StoreInfo.DailyLimit
	logrus.Debugf("店铺 %d 的每日上架限额为: %d，限制类型: %s", ctx.StoreInfo.ID, dailyLimit, ctx.StoreInfo.DailyLimitType)

	// 获取当前日期（格式：YYYY-MM-DD）
	currentDate := time.Now().Format("2006-01-02")

	// 根据店铺配置的限制类型计算增加的数量
	increment := h.calculateIncrement(ctx)
	if increment <= 0 {
		logrus.Warnf("计算增量失败，跳过计数更新")
		return nil
	}

	// 增加每日上架计数
	count := ctx.MemoryManager.DailyCountManager.IncrementCount(
		ctx.Task.TenantID,
		ctx.Task.StoreID,
		currentDate,
		increment,
	)

	logrus.Infof("店铺 %d 在 %s 的上架计数: %d (本次增加: %d, 类型: %s)",
		ctx.StoreInfo.ID, currentDate, count, increment, ctx.StoreInfo.DailyLimitType)

	// 检查是否超过限额
	if count > int64(dailyLimit) {
		logrus.Warnf("店铺 %d 在 %s 的上架数量(%d)已超过限额(%d)，将暂停上架", ctx.StoreInfo.ID, currentDate, count, dailyLimit)

		// 暂停店铺上架并清理相关缓存
		if err := h.pauseShopWithCacheCleanup(
			ctx,
			"超过每日上架限额",
			24*time.Hour, // 暂停24小时
		); err != nil {
			logrus.Errorf("暂停店铺上架并清理缓存失败: %v", err)
			// 暂停上架失败可能是网络或系统问题，可重试
		}

		// 记录日志
		logrus.Infof("已暂停店铺 %d 上架24小时并清理缓存，因为已超过每日限额 %d", ctx.StoreInfo.ID, dailyLimit)

	} else {
		logrus.Infof("店铺 %d 在 %s 的上架数量(%d)未超过限额(%d)", ctx.StoreInfo.ID, currentDate, count, dailyLimit)
	}

	return nil
}

// calculateIncrement 根据店铺配置的限制类型计算增量
func (h *SavePublishResultHandler) calculateIncrement(ctx *TaskContext) int64 {
	// 检查SheinResponse是否存在
	if ctx.SheinResponse == nil {
		logrus.Warn("SheinResponse为空，无法计算增量")
		return 0
	}

	switch ctx.StoreInfo.DailyLimitType {
	case "SPU":
		// SPU级别：每个产品算1个
		return 1
	case "SKC":
		// SKC级别：按SKC数量计算
		skcCount := int64(len(ctx.SheinResponse.Info.SKCList))
		logrus.Debugf("SKC计数: %d", skcCount)
		return skcCount
	case "SKU":
		// SKU级别：按所有SKU数量计算
		var skuCount int64
		for _, skc := range ctx.SheinResponse.Info.SKCList {
			skuCount += int64(len(skc.SKUList))
		}
		logrus.Debugf("SKU计数: %d", skuCount)
		return skuCount
	default:
		// 默认按SPU计算
		logrus.Warnf("未知的限制类型: %s，默认按SPU计算", ctx.StoreInfo.DailyLimitType)
		return 1
	}
}

// pauseShopWithCacheCleanup 暂停店铺并清理相关缓存
func (h *SavePublishResultHandler) pauseShopWithCacheCleanup(ctx *TaskContext, reason string, duration time.Duration) error {
	// 1. 删除客户端缓存
	if ctx.ShopClientMgr != nil {
		ctx.ShopClientMgr.RemoveClient(ctx.Task.TenantID, ctx.Task.StoreID)
		logrus.Infof("已删除店铺 %d:%d 的客户端缓存", ctx.Task.TenantID, ctx.Task.StoreID)
	}

	// 2. 设置暂停键，暂停该店铺
	ctx.MemoryManager.ShopPauseManager.PauseShop(
		ctx.Task.TenantID,
		ctx.Task.StoreID,
		reason,
		duration,
	)

	return nil
}

// int16Ptr 返回int16指针
func int16Ptr(i int16) *int16 {
	return &i
}

// updateTaskStatusToPublished 更新任务状态为已上架
func (h *SavePublishResultHandler) updateTaskStatusToPublished(ctx *TaskContext) error {
	// 检查必要的上下文信息
	if ctx.ManagementClientMgr == nil {
		logrus.Warn("管理客户端管理器未初始化，跳过状态更新")
		return nil
	}

	if ctx.Task == nil {
		logrus.Warn("任务信息未初始化，跳过状态更新")
		return nil
	}

	// 获取导入任务客户端
	importTaskClient := ctx.ManagementClientMgr.GetImportTaskClient()
	if importTaskClient == nil {
		logrus.Warn("导入任务客户端未初始化，跳过状态更新")
		return nil
	}

	// 解析任务ID
	var taskID int64
	if _, err := fmt.Sscanf(ctx.Task.ID, "%d", &taskID); err != nil {
		return fmt.Errorf("解析任务ID失败: %w", err)
	}

	// 构建更新请求
	req := &management_api.ProductImportTaskUpdateReqDTO{
		ID:     taskID,
		Status: model.TaskStatusPublished.Int16(),
	}

	// 异步更新状态
	go func() {
		if err := importTaskClient.UpdateTaskStatus(req); err != nil {
			logrus.Errorf("更新任务状态为已上架失败 (TaskID: %s): %v", ctx.Task.ID, err)
		} else {
			logrus.Infof("✅ 任务状态已更新为已上架 (TaskID: %s)", ctx.Task.ID)
		}
	}()

	return nil
}
