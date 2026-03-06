package product

import (
	"fmt"
	"math"
	"math/rand"

	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product"
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/management/api"
	temucontext "task-processor/internal/platforms/temu/context"

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

// Name 返回处理器名称
func (ph *PriceHandler) Name() string {
	return "价格处理器"
}

// Handle 处理任务（兼容pipeline.Handler接口）
func (ph *PriceHandler) Handle(ctx pipeline.TaskContext) error {
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return ph.HandleTemu(temuCtx)
}

// HandleTemu 处理任务（强类型上下文）
func (ph *PriceHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	// 这里可以根据需要进行价格相关的处理
	// 例如预加载利润规则等
	return nil
}

// CalculateDefaultPrice 计算默认价格（参考SHEIN的价格计算逻辑）
func (ph *PriceHandler) CalculateDefaultPrice(temuCtx *temucontext.TemuTaskContext, variant *model.Product) int {
	// 获取租户ID和店铺ID
	tenantID, storeID := ph.getTenantAndStoreID(temuCtx)

	// 获取利润规则
	profitRule, err := ph.profitRuleClient.GetProfitRule(&api.ProfitRuleReqDTO{
		TenantID: tenantID,
		StoreID:  storeID,
	})
	if err != nil {
		ph.logger.Warnf("获取利润规则失败: %v，使用默认倍数", err)
		// 使用默认倍数
		return ph.calculatePriceWithMultiplier(temuCtx, variant, 2.0)
	}

	// 检查规则是否为空
	if profitRule == nil {
		ph.logger.Warn("利润规则数据为空，使用默认倍数")
		return ph.calculatePriceWithMultiplier(temuCtx, variant, 2.0)
	}

	// 检查规则是否启用（Status = 1 表示禁用，0 表示启用）
	if profitRule.Status == 1 {
		ph.logger.Warnf("利润规则已禁用，使用默认倍数")
		return ph.calculatePriceWithMultiplier(temuCtx, variant, 2.0)
	}

	// 保存利润规则到强类型上下文
	temuCtx.ProfitRule = profitRule
	ph.logger.Infof("保存利润规则到context: ID=%d, Name=%s", profitRule.ID, profitRule.Name)

	// 获取产品原始价格（根据店铺配置的价格类型）
	originalPrice := ph.getSupplierCost(temuCtx, variant)

	// 应用利润规则：原始价格 * 售价倍数（参考SHEIN的计算方式）
	salePrice := math.Round(originalPrice*profitRule.SalePriceMultiplier*100) / 100

	ph.logger.Infof("应用利润规则 '%s' (%.2fx): $%.2f -> $%.2f",
		profitRule.Name, profitRule.SalePriceMultiplier, originalPrice, salePrice)

	// 转换为分并返回
	return int(math.Round(salePrice * 100))
}

// calculatePriceWithMultiplier 使用指定倍数计算价格
func (ph *PriceHandler) calculatePriceWithMultiplier(temuCtx *temucontext.TemuTaskContext, variant *model.Product, multiplier float64) int {
	originalPrice := ph.getSupplierCost(temuCtx, variant)
	salePrice := math.Round(originalPrice*multiplier*100) / 100
	return int(math.Round(salePrice * 100))
}

// CalculateVariantPrice 计算变体价格（使用上下文中的店铺配置）
func (ph *PriceHandler) CalculateVariantPrice(temuCtx *temucontext.TemuTaskContext, variant *model.Product) int {
	return ph.CalculateDefaultPrice(temuCtx, variant)
}

// getTenantAndStoreID 获取租户ID和店铺ID
func (ph *PriceHandler) getTenantAndStoreID(temuCtx *temucontext.TemuTaskContext) (int64, int64) {
	tenantID := int64(1) // 默认租户ID
	storeID := int64(1)  // 默认店铺ID

	task := temuCtx.GetTask()
	if task != nil && task.StoreID != 0 {
		storeID = int64(task.StoreID)
		if task.TenantID != 0 {
			tenantID = int64(task.TenantID)
		}
	}

	return tenantID, storeID
}

// getSupplierCost 获取供应商成本（根据店铺配置的价格类型）
func (ph *PriceHandler) getSupplierCost(temuCtx *temucontext.TemuTaskContext, variant *model.Product) float64 {
	// 获取店铺配置的价格类型(默认使用特价)
	priceType := ph.getPriceTypeFromContext(temuCtx)

	// 根据价格类型获取价格(包含运费) - 使用公共函数
	return product.GetProductPrice(variant, priceType)
}

// getPriceTypeFromContext 从上下文获取价格类型
func (ph *PriceHandler) getPriceTypeFromContext(temuCtx *temucontext.TemuTaskContext) string {
	// 从强类型上下文获取店铺信息
	if temuCtx.StoreInfo != nil && temuCtx.StoreInfo.PriceType != "" {
		return temuCtx.StoreInfo.PriceType
	}
	return "special" // 默认值
}

// GetDefaultStock 获取默认库存
func (ph *PriceHandler) GetDefaultStock(temuCtx *temucontext.TemuTaskContext) int {
	// 从强类型上下文获取店铺信息
	if temuCtx.StoreInfo != nil && temuCtx.StoreInfo.FixedStockCount != nil {
		stockCount := *temuCtx.StoreInfo.FixedStockCount

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
	randomStock := rand.Intn(1000) + 10
	ph.logger.Debugf("使用随机库存: %d", randomStock)
	return randomStock
}

// getStockFromStoreRespDTO 从 StoreRespDTO 获取库存（强类型，类型安全）
func (ph *PriceHandler) getStockFromStoreRespDTO(storeInfo *api.StoreRespDTO) int {
	if storeInfo.FixedStockCount != nil {
		stockCount := *storeInfo.FixedStockCount

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

	// 返回随机库存
	randomStock := rand.Intn(1000) + 10
	ph.logger.Debugf("使用随机库存: %d", randomStock)
	return randomStock
}

// getStockFromStoreInfo 从自定义 StoreInfo 获取库存
func (ph *PriceHandler) getStockFromStoreInfo(storeInfo *api.StoreRespDTO) int {
	if storeInfo.FixedStockCount != nil {
		stockCount := *storeInfo.FixedStockCount

		if stockCount == -1 {
			ph.logger.Debugf("店铺配置固定库存为-1，设置库存为0")
			return 0
		}

		if stockCount > 0 {
			ph.logger.Debugf("使用店铺配置的固定库存: %d", stockCount)
			return stockCount
		}
	}

	randomStock := rand.Intn(1000) + 10
	ph.logger.Debugf("使用随机库存: %d", randomStock)
	return randomStock
}

// GetPriceMultiplier 获取价格倍数（用于计算最大零售价格）
func (ph *PriceHandler) GetPriceMultiplier(temuCtx *temucontext.TemuTaskContext) float64 {
	// 获取租户ID和店铺ID
	tenantID, storeID := ph.getTenantAndStoreID(temuCtx)

	// 获取利润规则
	profitRule, err := ph.profitRuleClient.GetProfitRule(&api.ProfitRuleReqDTO{
		TenantID: tenantID,
		StoreID:  storeID,
	})
	if err != nil {
		ph.logger.Warnf("获取利润规则失败: %v，使用默认倍数2.0", err)
		return 3.0
	}

	// 检查规则是否为空
	if profitRule == nil {
		ph.logger.Warn("利润规则数据为空，使用默认倍数2.0")
		return 3.0
	}

	// 检查规则是否启用（Status = 1 表示禁用，0 表示启用）
	if profitRule.Status == 1 {
		ph.logger.Warnf("利润规则已禁用，使用默认倍数2.0")
		return 3.0
	}

	ph.logger.Infof("使用利润规则 '%s' 的倍数: %.2fx", profitRule.Name, profitRule.SalePriceMultiplier)
	return profitRule.SalePriceMultiplier
}
