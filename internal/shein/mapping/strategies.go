// Package mapping 提供 SHEIN 平台商品映射功能
package mapping

import (
	"fmt"
	"time"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/shein/api/product"
	shein_product "task-processor/internal/shein/api/product"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// ProductBasedRepairStrategy 基于产品信息的修复策略
type ProductBasedRepairStrategy struct {
	mappingBuilder *MappingBuilder
	logger         *logrus.Entry
}

// NewProductBasedRepairStrategy 创建基于产品信息的修复策略
func NewProductBasedRepairStrategy(
	mappingClient managementapi.ProductImportMappingAPI,
	productAPI shein_product.ProductAPI,
) MappingRepairStrategy {
	return &ProductBasedRepairStrategy{
		mappingBuilder: NewMappingBuilder(mappingClient),
		logger:         logger.GetGlobalLogger("ProductBasedRepair"),
	}
}

// CanRepair 判断是否可以修复
func (s *ProductBasedRepairStrategy) CanRepair(ctx *MappingRepairContext) bool {
	// 需要有SKU编码和店铺信息
	return ctx.Request.SkuCode != "" && ctx.StoreInfo != nil
}

// Repair 执行修复
func (s *ProductBasedRepairStrategy) Repair(ctx *MappingRepairContext) (*MappingRepairResult, error) {
	s.logger.WithField("sku_code", ctx.Request.SkuCode).Info("开始基于产品信息的修复")

	// 使用统一的映射关系创建函数
	mappingInfo, err := s.mappingBuilder.CreateMappingFromContext(
		ctx,
		fmt.Sprintf("基于产品信息的自动修复: %s", ctx.Request.Reason),
	)

	if err != nil {
		s.logger.WithError(err).WithField("sku_code", ctx.Request.SkuCode).Error("基于产品信息的修复失败")
		return &MappingRepairResult{
			SkuCode:    ctx.Request.SkuCode,
			Success:    false,
			Error:      fmt.Sprintf("基于产品信息的修复失败: %v", err),
			RepairTime: time.Now(),
		}, nil
	}

	s.logger.WithFields(logrus.Fields{
		"sku_code":   ctx.Request.SkuCode,
		"mapping_id": mappingInfo.ID,
	}).Info("基于产品信息的修复成功")

	return &MappingRepairResult{
		SkuCode:     ctx.Request.SkuCode,
		Success:     true,
		MappingInfo: mappingInfo,
		RepairTime:  time.Now(),
	}, nil
}

// GetStrategyName 获取策略名称
func (s *ProductBasedRepairStrategy) GetStrategyName() string {
	return "ProductBasedRepair"
}

// HistoryBasedRepairStrategy 基于历史记录的修复策略
type HistoryBasedRepairStrategy struct {
	mappingBuilder *MappingBuilder
	logger         *logrus.Entry
}

// NewHistoryBasedRepairStrategy 创建基于历史记录的修复策略
func NewHistoryBasedRepairStrategy(
	mappingClient managementapi.ProductImportMappingAPI,
) MappingRepairStrategy {
	return &HistoryBasedRepairStrategy{
		mappingBuilder: NewMappingBuilder(mappingClient),
		logger:         logger.GetGlobalLogger("HistoryBasedRepair"),
	}
}

// CanRepair 判断是否可以修复
func (s *HistoryBasedRepairStrategy) CanRepair(ctx *MappingRepairContext) bool {
	// 需要有SPU信息来查找相关的历史记录
	return ctx.Request.SpuCode != "" || ctx.Request.SpuName != ""
}

// Repair 执行修复
func (s *HistoryBasedRepairStrategy) Repair(ctx *MappingRepairContext) (*MappingRepairResult, error) {
	s.logger.WithField("sku_code", ctx.Request.SkuCode).Info("开始基于历史记录的修复")

	// 这里可以实现基于历史记录的修复逻辑
	// 例如：查找同一SPU下的其他SKU映射关系，复制相关信息

	// 目前先返回不支持
	return &MappingRepairResult{
		SkuCode:    ctx.Request.SkuCode,
		Success:    false,
		Error:      "基于历史记录的修复策略暂未实现",
		RepairTime: time.Now(),
	}, nil
}

// GetStrategyName 获取策略名称
func (s *HistoryBasedRepairStrategy) GetStrategyName() string {
	return "HistoryBasedRepair"
}

// SmartRepairStrategy 智能修复策略（结合多种信息源）
type SmartRepairStrategy struct {
	mappingBuilder   *MappingBuilder
	productAPI       shein_product.ProductAPI
	inventoryManager *shein_product.InventoryManager
	priceManager     *shein_product.PriceManager
	logger           *logrus.Entry
}

// NewSmartRepairStrategy 创建智能修复策略
func NewSmartRepairStrategy(
	mappingClient managementapi.ProductImportMappingAPI,
	productAPI shein_product.ProductAPI,
	inventoryManager *shein_product.InventoryManager,
	priceManager *shein_product.PriceManager,
) MappingRepairStrategy {
	return &SmartRepairStrategy{
		mappingBuilder:   NewMappingBuilder(mappingClient),
		productAPI:       productAPI,
		inventoryManager: inventoryManager,
		priceManager:     priceManager,
		logger:           logger.GetGlobalLogger("SmartRepair"),
	}
}

// CanRepair 判断是否可以修复
func (s *SmartRepairStrategy) CanRepair(ctx *MappingRepairContext) bool {
	// 智能策略可以处理大部分情况
	return ctx.Request.SkuCode != "" && ctx.StoreInfo != nil
}

// Repair 执行修复
func (s *SmartRepairStrategy) Repair(ctx *MappingRepairContext) (*MappingRepairResult, error) {
	s.logger.WithField("sku_code", ctx.Request.SkuCode).Info("开始智能修复")

	// 1. 尝试获取SKU的详细信息
	skuInfo, err := s.getSkuDetailInfo(ctx.Request.SkuCode, ctx.Request.SpuName)
	if err != nil {
		s.logger.WithError(err).WithField("sku_code", ctx.Request.SkuCode).Warn("获取SKU详细信息失败")
	}

	// 2. 构建增强的映射创建选项
	options := s.buildEnhancedMappingOptions(ctx, skuInfo)

	// 3. 使用统一的映射关系创建函数
	mappingInfo, err := s.mappingBuilder.CreateMappingRelation(options)
	if err != nil {
		return &MappingRepairResult{
			SkuCode:    ctx.Request.SkuCode,
			Success:    false,
			Error:      fmt.Sprintf("智能修复失败: %v", err),
			RepairTime: time.Now(),
		}, nil
	}

	s.logger.WithFields(logrus.Fields{
		"sku_code":   ctx.Request.SkuCode,
		"mapping_id": mappingInfo.ID,
	}).Info("智能修复成功")

	return &MappingRepairResult{
		SkuCode:     ctx.Request.SkuCode,
		Success:     true,
		MappingInfo: mappingInfo,
		RepairTime:  time.Now(),
	}, nil
}

// GetStrategyName 获取策略名称
func (s *SmartRepairStrategy) GetStrategyName() string {
	return "SmartRepair"
}

// getSkuDetailInfo 获取SKU详细信息
func (s *SmartRepairStrategy) getSkuDetailInfo(skuCode, spuName string) (*product.SkuInfo, error) {
	// 这里可以实现通过API获取SKU详细信息的逻辑
	// 目前返回nil，表示无法获取详细信息
	return nil, fmt.Errorf("SKU详细信息获取功能暂未实现")
}

// buildEnhancedMappingOptions 构建增强的映射创建选项
func (s *SmartRepairStrategy) buildEnhancedMappingOptions(
	ctx *MappingRepairContext,
	skuInfo *product.SkuInfo,
) *MappingCreateOptions {
	options := &MappingCreateOptions{
		TenantID: ctx.Request.TenantID,
		StoreID:  ctx.Request.StoreID,
		SkuCode:  ctx.Request.SkuCode,
		SpuCode:  ctx.Request.SpuCode,
		SpuName:  ctx.Request.SpuName,
		Region:   s.determineRegion(ctx.StoreInfo),
		Reason:   fmt.Sprintf("智能修复创建: %s", ctx.Request.Reason),
	}

	// 如果有SKU详细信息，设置更多字段
	if skuInfo != nil {
		s.logger.WithField("sku_code", ctx.Request.SkuCode).Debug("使用SKU详细信息增强映射关系")
		// 这里可以根据SKU信息设置成本价等字段
		// if skuInfo.CostPrice > 0 {
		//     options.CostPrice = &skuInfo.CostPrice
		// }
		// if skuInfo.SupplierSku != "" {
		//     options.SupplierSku = skuInfo.SupplierSku
		// }
	}

	// 如果有历史映射信息，可以复用一些配置
	// 注意：当前上下文中没有历史映射信息，这里预留扩展接口
	// 未来可以通过查询相同SPU的其他SKU映射关系来获取历史信息

	return options
}

// determineRegion 确定区域
func (s *SmartRepairStrategy) determineRegion(storeInfo *managementapi.StoreRespDTO) string {
	if storeInfo != nil && storeInfo.Region != "" {
		return storeInfo.Region
	}
	return "US"
}
