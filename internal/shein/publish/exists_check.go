// Package publish 提供SHEIN平台产品存在性检查处理器
package publish

import (
	"task-processor/internal/core/logger"
	"fmt"

	management_api "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"

)

// ProductExistsCheckHandler 产品存在性检查处理器
type ProductExistsCheckHandler struct {
	checker *PublishProductChecker
}

// NewProductExistsCheckHandler 创建新的产品存在性检查处理器
func NewProductExistsCheckHandler() *ProductExistsCheckHandler {
	return &ProductExistsCheckHandler{
		checker: NewPublishProductChecker(),
	}
}

// Name 返回处理器名称
func (h *ProductExistsCheckHandler) Name() string {
	return "产品存在性检查"
}

// Handle 执行产品存在性检查
func (h *ProductExistsCheckHandler) Handle(ctx *shein.TaskContext) error {
	logger.GetGlobalLogger("shein/publish").Info("🔍 开始检查产品是否已上架...")

	// 检查必要的上下文信息
	if ctx.ManagementClientMgr == nil {
		logger.GetGlobalLogger("shein/publish").Warn("管理客户端管理器未初始化，跳过产品存在性检查")
		return nil
	}

	if ctx.Task == nil {
		return shein.NewNonRetryableError("任务信息未初始化", nil)
	}

	// 获取产品导入映射客户端
	mappingClient := ctx.ManagementClientMgr.GetProductImportMappingClient()
	if mappingClient == nil {
		logger.GetGlobalLogger("shein/publish").Warn("产品导入映射客户端未初始化，跳过产品存在性检查")
		return nil
	}

	// 检查主产品是否已上架
	if err := h.checkMainProduct(ctx, mappingClient); err != nil {
		return err
	}

	// 检查变体产品是否已上架
	if err := h.checkVariantProducts(ctx, mappingClient); err != nil {
		return err
	}

	logger.GetGlobalLogger("shein/publish").Info("✅ 产品存在性检查完成")
	return nil
}

// checkMainProduct 检查主产品是否已上架
func (h *ProductExistsCheckHandler) checkMainProduct(ctx *shein.TaskContext, mappingClient management_api.ProductImportMappingAPI) error {
	if ctx.Task.ProductID == "" {
		logger.GetGlobalLogger("shein/publish").Debug("主产品ID为空，跳过主产品检查")
		return nil
	}

	req := &management_api.ProductImportMappingCheckReqDTO{
		StoreId:   ctx.Task.StoreID,
		Platform:  ctx.Task.Platform,
		Region:    ctx.Task.Region,
		ProductId: ctx.Task.ProductID,
	}

	exists, err := mappingClient.CheckProductExists(req)
	if err != nil {
		logger.GetGlobalLogger("shein/publish").Errorf("❌ 检查主产品 %s 是否已上架失败: %v", ctx.Task.ProductID, err)
		// 检查失败可能是网络问题，可重试
		return shein.NewRetryableError("检查主产品是否已上架失败", err)
	}

	if exists {
		logger.GetGlobalLogger("shein/publish").Warnf("⚠️ 主产品 %s 已经上架过，跳过本次上架", ctx.Task.ProductID)
		return shein.NewNonRetryableError(fmt.Sprintf("主产品 %s 已经上架过", ctx.Task.ProductID), nil)
	}

	logger.GetGlobalLogger("shein/publish").Infof("✅ 主产品 %s 未上架，可以继续上架流程", ctx.Task.ProductID)
	return nil
}

// checkVariantProducts 检查变体产品是否已上架
func (h *ProductExistsCheckHandler) checkVariantProducts(ctx *shein.TaskContext, mappingClient management_api.ProductImportMappingAPI) error {
	if ctx.Variants == nil || len(*ctx.Variants) == 0 {
		logger.GetGlobalLogger("shein/publish").Debug("无变体产品，跳过变体检查")
		return nil
	}

	logger.GetGlobalLogger("shein/publish").Infof("开始检查 %d 个变体产品...", len(*ctx.Variants))

	for i, variant := range *ctx.Variants {
		if variant.Asin == "" {
			logger.GetGlobalLogger("shein/publish").Debugf("变体[%d/%d] ASIN为空，跳过", i+1, len(*ctx.Variants))
			continue
		}

		if err := h.checkSingleVariant(ctx, mappingClient, variant, i+1, len(*ctx.Variants)); err != nil {
			// 单个变体检查失败不影响整体流程，记录日志后继续
			logger.GetGlobalLogger("shein/publish").Warnf("变体[%d/%d] %s 检查失败: %v", i+1, len(*ctx.Variants), variant.Asin, err)
		}
	}

	return nil
}

// checkSingleVariant 检查单个变体产品
func (h *ProductExistsCheckHandler) checkSingleVariant(ctx *shein.TaskContext, mappingClient management_api.ProductImportMappingAPI, variant model.Product, index, total int) error {
	req := &management_api.ProductImportMappingCheckReqDTO{
		StoreId:   ctx.Task.StoreID,
		Platform:  ctx.Task.Platform,
		Region:    ctx.Task.Region,
		ProductId: variant.Asin,
	}

	exists, err := mappingClient.CheckProductExists(req)
	if err != nil {
		logger.GetGlobalLogger("shein/publish").Errorf("❌ 检查变体[%d/%d] %s 是否已上架失败: %v", index, total, variant.Asin, err)
		return err
	}

	if exists {
		logger.GetGlobalLogger("shein/publish").Warnf("⚠️ 变体[%d/%d] %s 已经上架过", index, total, variant.Asin)
		// 标记该变体已被筛选掉
		ctx.SetVariantFiltered(variant.Asin, true, fmt.Sprintf("变体 %s 已经上架过", variant.Asin))
	} else {
		logger.GetGlobalLogger("shein/publish").Debugf("✅ 变体[%d/%d] %s 未上架", index, total, variant.Asin)
	}

	return nil
}
