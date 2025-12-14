package handlers

import (
	"math"
	"math/rand"

	"task-processor/common/amazon/model"
	"task-processor/common/management/api"
	"task-processor/common/pipeline"

	"github.com/sirupsen/logrus"
)

// PriceHandler 价格处理器（参考SHEIN的简洁设计）
type PriceHandler struct {
	profitRuleClient api.ProfitRuleAPI
	logger           *logrus.Entry
}

// NewPriceHandler 创建新的价格处理器
func NewPriceHandler(profitRuleClient api.ProfitRuleAPI) *PriceHandler {
	return &PriceHandler{
		profitRuleClient: profitRuleClient,
		logger:           logrus.WithField("handler", "PriceHandler"),
	}
}

// CalculateDefaultPrice 计算默认价格（参考SHEIN的价格计算逻辑）
func (ph *PriceHandler) CalculateDefaultPrice(ctx *pipeline.TaskContext) int {
	// 获取租户ID和店铺ID
	tenantID, storeID := ph.getTenantAndStoreID(ctx)

	// 获取利润规则
	profitRule, err := ph.profitRuleClient.GetProfitRule(&api.ProfitRuleReqDTO{
		TenantID: tenantID,
		StoreID:  storeID,
	})
	if err != nil {
		ph.logger.Warnf("获取利润规则失败: %v，使用默认倍数", err)
		// 使用默认倍数
		return ph.calculatePriceWithMultiplier(ctx.AmazonProduct, 2.0, ctx)
	}

	// 检查规则是否为空
	if profitRule == nil {
		ph.logger.Warn("利润规则数据为空，使用默认倍数")
		return ph.calculatePriceWithMultiplier(ctx.AmazonProduct, 2.0, ctx)
	}

	// 检查规则是否启用（Status = 1 表示禁用，0 表示启用）
	if profitRule.Status == 1 {
		ph.logger.Warnf("利润规则已禁用，使用默认倍数")
		return ph.calculatePriceWithMultiplier(ctx.AmazonProduct, 2.0, ctx)
	}

	// 保存利润规则到 context，供后续保存使用
	ctx.SetData("profit_rule", profitRule)
	ph.logger.Infof("保存利润规则到context: ID=%d, Name=%s", profitRule.ID, profitRule.Name)

	// 获取产品原始价格（根据店铺配置的价格类型）
	originalPrice := ph.getSupplierCost(ctx.AmazonProduct, ctx)

	// 应用利润规则：原始价格 * 售价倍数（参考SHEIN的计算方式）
	salePrice := math.Round(originalPrice*profitRule.SalePriceMultiplier*100) / 100

	ph.logger.Infof("应用利润规则 '%s' (%.2fx): $%.2f -> $%.2f",
		profitRule.Name, profitRule.SalePriceMultiplier, originalPrice, salePrice)

	// 转换为分并返回
	return int(math.Round(salePrice * 100))
}

// calculatePriceWithMultiplier 使用指定倍数计算价格
func (ph *PriceHandler) calculatePriceWithMultiplier(product *model.Product, multiplier float64, ctx *pipeline.TaskContext) int {
	originalPrice := ph.getSupplierCost(product, ctx)
	salePrice := math.Round(originalPrice*multiplier*100) / 100
	return int(math.Round(salePrice * 100))
}

// CalculateVariantPrice 计算变体价格（使用上下文中的店铺配置）
func (ph *PriceHandler) CalculateVariantPrice(ctx *pipeline.TaskContext, variant *model.Product) int {
	tempCtx := &pipeline.TaskContext{
		Task:          ctx.Task,
		AmazonProduct: variant,
		StoreInfo:     ctx.StoreInfo,
		Data:          ctx.Data, // 复用原始context的Data，保留profit_rule等数据
	}
	return ph.CalculateDefaultPrice(tempCtx)
}

// CalculateVariantPriceWithStoreConfig 使用店铺配置计算变体价格（已废弃，使用 CalculateVariantPrice 代替）
// Deprecated: 使用 CalculateVariantPrice 代替
func (ph *PriceHandler) CalculateVariantPriceWithStoreConfig(ctx *pipeline.TaskContext, variant *model.Product) int {
	return ph.CalculateVariantPrice(ctx, variant)
}

// getTenantAndStoreID 获取租户ID和店铺ID
func (ph *PriceHandler) getTenantAndStoreID(ctx *pipeline.TaskContext) (int64, int64) {
	tenantID := int64(1) // 默认租户ID
	storeID := int64(1)  // 默认店铺ID

	if ctx.Task != nil && ctx.Task.StoreID != 0 {
		storeID = int64(ctx.Task.StoreID)
		if ctx.Task.TenantID != 0 {
			tenantID = int64(ctx.Task.TenantID)
		}
	}

	return tenantID, storeID
}

// getSupplierCost 获取供应商成本（根据店铺配置的价格类型）
func (ph *PriceHandler) getSupplierCost(product *model.Product, ctx *pipeline.TaskContext) float64 {
	// 检查产品是否为空
	if product == nil {
		ph.logger.Warn("产品信息为空，返回默认价格0")
		return 0
	}

	// 获取店铺配置的价格类型(默认使用特价)
	priceType := "special"
	if ctx != nil && ctx.StoreInfo != nil && ctx.StoreInfo.PriceType != "" {
		priceType = ctx.StoreInfo.PriceType
	}

	// 根据价格类型获取价格(包含运费)
	return getProductPrice(product, priceType)
}

// GetDefaultStock 获取默认库存（参考SHEIN从店铺信息获取）
func (ph *PriceHandler) GetDefaultStock(ctx *pipeline.TaskContext) int {
	// 优先从店铺信息获取固定库存数量
	if ctx != nil && ctx.StoreInfo != nil && ctx.StoreInfo.FixedStockCount != nil {
		stockCount := *ctx.StoreInfo.FixedStockCount

		// 如果固定库存为-1，设置库存数量为0
		if stockCount == -1 {
			ph.logger.Debugf("店铺配置固定库存为-1，设置库存为0")
			return 0
		}

		if stockCount > 0 {
			ph.logger.Debugf("使用店铺配置的固定库存: %d", stockCount)
			return stockCount
		}
	}

	// 如果店铺未配置或配置为0，返回随机库存（10-1009之间）
	// 参考 SHEIN: rand.Intn(1000) + 10
	randomStock := rand.Intn(1000) + 10
	ph.logger.Debugf("使用随机库存: %d", randomStock)
	return randomStock
}

// GetPriceMultiplier 获取价格倍数（用于计算最大零售价格）
func (ph *PriceHandler) GetPriceMultiplier(ctx *pipeline.TaskContext) float64 {
	// 获取租户ID和店铺ID
	tenantID, storeID := ph.getTenantAndStoreID(ctx)

	// 获取利润规则
	profitRule, err := ph.profitRuleClient.GetProfitRule(&api.ProfitRuleReqDTO{
		TenantID: tenantID,
		StoreID:  storeID,
	})
	if err != nil {
		ph.logger.Warnf("获取利润规则失败: %v，使用默认倍数2.0", err)
		return 2.0
	}

	// 检查规则是否为空
	if profitRule == nil {
		ph.logger.Warn("利润规则数据为空，使用默认倍数2.0")
		return 2.0
	}

	// 检查规则是否启用（Status = 1 表示禁用，0 表示启用）
	if profitRule.Status == 1 {
		ph.logger.Warnf("利润规则已禁用，使用默认倍数2.0")
		return 2.0
	}

	ph.logger.Infof("使用利润规则 '%s' 的倍数: %.2fx", profitRule.Name, profitRule.SalePriceMultiplier)
	return profitRule.SalePriceMultiplier
}

// getProductPrice 获取产品价格(包含运费)
func getProductPrice(amazonProduct *model.Product, priceType string) float64 {
	if amazonProduct == nil {
		return 0
	}

	// 获取运费
	freight := getFreight(amazonProduct)

	var price float64
	switch priceType {
	case "special":
		// 特价
		price = amazonProduct.FinalPrice
	case "original":
		// 原价
		if amazonProduct.PricesBreakdown.ListPrice != nil {
			price = *amazonProduct.PricesBreakdown.ListPrice
		} else {
			price = amazonProduct.InitialPrice
		}
	default:
		// 默认使用特价
		price = amazonProduct.FinalPrice
	}

	return freight + price
}

// getFreight 获取运费
func getFreight(amazonProduct *model.Product) float64 {
	// TODO: 从 delivery 信息中提取运费
	// 目前假设运费为0
	return 0
}
