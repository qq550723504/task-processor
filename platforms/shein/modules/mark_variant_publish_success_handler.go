package modules

import (
	"fmt"
	"strconv"
	"task-processor/common"
	management_api "task-processor/common/management/api"

	"github.com/sirupsen/logrus"
)

// MarkVariantPublishSuccessHandler 标记变体发布成功处理器
type MarkVariantPublishSuccessHandler struct {
}

// NewMarkVariantBuildSuccessHandler 创建新的标记变体发布成功处理器
func NewMarkVariantPublishSuccessHandler() *MarkVariantPublishSuccessHandler {
	return &MarkVariantPublishSuccessHandler{}
}

// Name 返回处理器名称
func (h *MarkVariantPublishSuccessHandler) Name() string {
	return "开始标记产品发布成功"
}

// Handle 执行标记变体发布成功处理
func (h *MarkVariantPublishSuccessHandler) Handle(ctx *TaskContext) error {
	logrus.Infof("=== 开始标记产品发布成功 ===")

	// 检查必要的上下文字段
	if ctx == nil {
		logrus.Errorf("❌ TaskContext 为 nil")
		return fmt.Errorf("TaskContext 为 nil")
	}

	// 检查管理客户端是否可用
	if ctx.ManagementClientMgr == nil {
		logrus.Warn("管理客户端管理器未初始化，跳过状态更新")
		return nil
	}

	// 标记成功发布的变体
	if ctx.Task != nil && ctx.SheinResponse != nil {
		// 遍历发布后的响应数据来构建任务数据
		skus := []string{}
		for _, skc := range ctx.SheinResponse.Info.SKCList {
			for _, sku := range skc.SKUList {
				skus = append(skus, sku.SupplierSKU)
			}
		}

		logrus.Infof("📊 开始标记 %d 个 SKU 为已发布", len(skus))
		successCount := 0
		failCount := 0

		for _, sku := range skus {
			// 使用GetAsinBySku函数从AsinSkuMap中反向查找原始ASIN
			if asin := GetAsinBySku(ctx, sku); asin != "" {
				if err := h.markVariantPublished(ctx, asin, sku); err != nil {
					logrus.Errorf("标记变体发布成功失败 (ASIN: %s, SKU: %s): %v", asin, sku, err)
					failCount++
				} else {
					successCount++
				}
			} else {
				logrus.Warnf("⚠️ 未找到SKU %s 对应的ASIN", sku)
				failCount++
			}
		}

		logrus.Infof("📊 标记完成: 成功 %d 个, 失败 %d 个, 总计 %d 个", successCount, failCount, len(skus))
	} else {
		logrus.Warnf("⚠️ 任务信息或Shein响应不可用，无法标记任务完成")
	}

	// 处理被筛选掉的变体
	if ctx.UnFilteredVariants != nil && len(*ctx.UnFilteredVariants) > 0 {
		for _, variant := range *ctx.UnFilteredVariants {
			filterInfo := ctx.GetVariantFilterInfo(variant.Asin)
			if filterInfo != nil && filterInfo.FilteredOut {
				if err := h.markVariantFailed(ctx, variant.Asin, filterInfo.FilterReason); err != nil {
					logrus.Errorf("标记变体失败失败 (ASIN: %s): %v", variant.Asin, err)
				}
			}
		}
	}

	// 更新任务状态为已上架
	if err := h.updateTaskStatusToPublished(ctx); err != nil {
		logrus.Warnf("更新任务状态为已上架失败: %v", err)
	}

	return nil
}

// markVariantPublished 标记变体为已发布
func (h *MarkVariantPublishSuccessHandler) markVariantPublished(ctx *TaskContext, asin, sku string) error {
	mappingClient := ctx.ManagementClientMgr.GetProductImportMappingClient()
	if mappingClient == nil {
		return fmt.Errorf("产品导入映射客户端未初始化")
	}

	// 解析任务ID
	taskID, err := strconv.ParseInt(ctx.Task.ID, 10, 64)
	if err != nil {
		return fmt.Errorf("转换任务ID失败: %v", err)
	}

	// 获取变体信息
	variant := GetVariantByAsinFromVariants(ctx.Variants, asin)
	if variant == nil {
		return fmt.Errorf("未找到ASIN %s 对应的变体", asin)
	}

	// 计算成本价
	costPrice := GetProductPrice(variant, ctx.FilterRule.PriceType)

	// 构建创建请求
	status := common.TaskStatusPublished.Int16()
	createReq := &management_api.ProductImportMappingCreateReqDTO{
		TenantID:     ctx.Task.TenantID,
		ImportTaskId: taskID,
		StoreId:      ctx.Task.StoreID,
		Platform:     ctx.Task.Platform,
		Region:       ctx.Task.Region,
		ProductId:    asin,
		Sku:          &sku,
		CostPrice:    &costPrice,
		Status:       &status,
	}

	// 设置父产品ID
	if ctx.AmazonProduct != nil && ctx.AmazonProduct.ParentAsin != "" {
		createReq.ParentProductId = &ctx.AmazonProduct.ParentAsin
	}

	// 设置平台产品ID
	if ctx.ProductData != nil && ctx.ProductData.SPUName != "" {
		createReq.PlatformParentProductId = &ctx.ProductData.SPUName
	}

	// 设置筛选规则
	if ctx.FilterRule != nil {
		createReq.FilterRuleId = &ctx.FilterRule.ID
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

	// 设置利润规则
	if ctx.ProfitRule != nil {
		createReq.ProfitRuleId = &ctx.ProfitRule.ID
		salePriceMultiplier := fmt.Sprintf("%.2f", ctx.ProfitRule.SalePriceMultiplier)
		createReq.SalePriceMultiplier = &salePriceMultiplier

		if ctx.ProfitRule.DiscountPriceMultiplier > 0 {
			discountPriceMultiplier := fmt.Sprintf("%.2f", ctx.ProfitRule.DiscountPriceMultiplier)
			createReq.DiscountPriceMultiplier = &discountPriceMultiplier
		}
	}

	// 调用API创建产品导入映射关系
	id, err := mappingClient.CreateProductImportMapping(createReq)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"asin":                       asin,
			"sku":                        sku,
			"platform_parent_product_id": createReq.PlatformParentProductId,
			"error":                      err.Error(),
		}).Errorf("❌ 创建产品导入映射关系失败")
		return fmt.Errorf("创建产品导入映射关系失败: %v", err)
	}

	logrus.WithFields(logrus.Fields{
		"id":                         id,
		"asin":                       asin,
		"sku":                        sku,
		"platform_parent_product_id": createReq.PlatformParentProductId,
	}).Infof("✅ 成功标记变体为已发布")
	return nil
}

// markVariantFailed 标记变体为失败
func (h *MarkVariantPublishSuccessHandler) markVariantFailed(ctx *TaskContext, asin, reason string) error {
	mappingClient := ctx.ManagementClientMgr.GetProductImportMappingClient()
	if mappingClient == nil {
		return fmt.Errorf("产品导入映射客户端未初始化")
	}

	// 解析任务ID
	taskID, err := strconv.ParseInt(ctx.Task.ID, 10, 64)
	if err != nil {
		return fmt.Errorf("转换任务ID失败: %v", err)
	}

	// 获取变体信息
	variant := GetVariantByAsinFromVariants(ctx.UnFilteredVariants, asin)
	if variant == nil {
		return fmt.Errorf("未找到ASIN %s 对应的变体", asin)
	}

	// 计算成本价
	costPrice := GetProductPrice(variant, ctx.FilterRule.PriceType)

	// 构建创建请求
	status := common.TaskStatusCrawlFailed.Int16()
	remark := reason
	createReq := &management_api.ProductImportMappingCreateReqDTO{
		TenantID:     ctx.Task.TenantID,
		ImportTaskId: taskID,
		StoreId:      ctx.Task.StoreID,
		Platform:     ctx.Task.Platform,
		Region:       ctx.Task.Region,
		ProductId:    asin,
		CostPrice:    &costPrice,
		Status:       &status,
		Remark:       &remark,
	}

	// 设置父产品ID
	if ctx.AmazonProduct != nil && ctx.AmazonProduct.ParentAsin != "" {
		createReq.ParentProductId = &ctx.AmazonProduct.ParentAsin
	}

	// 设置筛选规则
	if ctx.FilterRule != nil {
		createReq.FilterRuleId = &ctx.FilterRule.ID
	}

	// 设置利润规则
	if ctx.ProfitRule != nil {
		createReq.ProfitRuleId = &ctx.ProfitRule.ID
	}

	// 调用API创建产品导入映射关系
	id, err := mappingClient.CreateProductImportMapping(createReq)
	if err != nil {
		return fmt.Errorf("创建产品导入映射关系失败: %v", err)
	}

	logrus.Infof("❌ 成功标记变体为失败 (ID: %d, ASIN: %s, Reason: %s)", id, asin, reason)
	return nil
}

// updateTaskStatusToPublished 更新任务状态为已上架
func (h *MarkVariantPublishSuccessHandler) updateTaskStatusToPublished(ctx *TaskContext) error {
	// 获取导入任务客户端
	importTaskClient := ctx.ManagementClientMgr.GetImportTaskClient()
	if importTaskClient == nil {
		logrus.Warn("导入任务客户端未初始化，跳过状态更新")
		return nil
	}

	// 解析任务ID
	taskID, err := strconv.ParseInt(ctx.Task.ID, 10, 64)
	if err != nil {
		return fmt.Errorf("解析任务ID失败: %v", err)
	}

	// 构建更新请求
	req := &management_api.ProductImportTaskUpdateReqDTO{
		ID:     taskID,
		Status: common.TaskStatusPublished.Int16(),
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
