// Package adapter 提供TEMU平台的核价适配器实现。
package adapter

import (
	"context"
	"fmt"
	"task-processor/internal/common/pricing/model"
	"task-processor/internal/common/pricing/service"

	"github.com/sirupsen/logrus"
)

// TemuAdapter TEMU平台适配器
type TemuAdapter struct {
	costProvider service.CostPriceProvider
	logger       *logrus.Entry
}

// NewTemuAdapter 创建TEMU适配器
func NewTemuAdapter(costProvider service.CostPriceProvider) *TemuAdapter {
	return &TemuAdapter{
		costProvider: costProvider,
		logger:       logrus.WithField("component", "TemuAdapter"),
	}
}

// GetPlatformName 获取平台名称
func (a *TemuAdapter) GetPlatformName() string {
	return "temu"
}

// PreprocessContext 预处理核价上下文
func (a *TemuAdapter) PreprocessContext(ctx context.Context, pricingCtx *model.PricingContext) error {
	// TEMU特有的预处理逻辑

	// 如果没有原始成本价，尝试通过成本价提供者获取
	if !pricingCtx.HasValidCostPrice() {
		originCostPrice, err := a.costProvider.GetOriginCostPrice(
			ctx, pricingCtx.ProductID, pricingCtx.StoreID)
		if err != nil {
			a.logger.Warnf("获取成本价失败: %v", err)
		} else {
			pricingCtx.OriginCostPrice = originCostPrice
			a.logger.Infof("TEMU商品 %s 获取成本价: %.2f",
				pricingCtx.ProductName, originCostPrice)
		}
	}

	// 处理TEMU特有的平台数据
	if platformData, ok := pricingCtx.PlatformData.(map[string]interface{}); ok {
		// 处理销量提升场景
		if salesBoost, exists := platformData["sales_boost"]; exists && salesBoost.(bool) {
			a.logger.Infof("TEMU商品 %s 为销量提升场景", pricingCtx.ProductName)
			// 可以在这里添加销量提升场景的特殊处理逻辑
		}
	}

	return nil
}

// PostprocessResult 后处理核价结果
func (a *TemuAdapter) PostprocessResult(ctx context.Context, result *model.PricingResult) error {
	// TEMU特有的后处理逻辑

	// 为TEMU平台添加特定的结果数据
	temuData := map[string]interface{}{
		"platform":     "temu",
		"processed_at": result.Timestamp,
	}

	// 如果是重新申诉，添加TEMU特有的申诉信息
	if result.Action == model.ActionReappeal {
		temuData["reappeal_enabled"] = true
		temuData["reappeal_reason"] = "价格不满足利润率要求，重新议价"
	}

	result.PlatformData = temuData

	a.logger.Debugf("TEMU商品 %s 后处理完成", result.ProductName)
	return nil
}

// ExecuteDecision 执行核价决策
func (a *TemuAdapter) ExecuteDecision(ctx context.Context, result *model.PricingResult) error {
	// TEMU平台特有的决策执行逻辑

	switch result.Action {
	case model.ActionAccept:
		return a.executeAccept(ctx, result)
	case model.ActionReject:
		return a.executeReject(ctx, result)
	case model.ActionReappeal:
		return a.executeReappeal(ctx, result)
	case model.ActionSkip:
		a.logger.Infof("TEMU商品 %s 跳过处理: %s", result.ProductName, result.Reason)
		return nil
	default:
		return fmt.Errorf("不支持的决策动作: %s", result.Action)
	}
}

// executeAccept 执行接受决策
func (a *TemuAdapter) executeAccept(ctx context.Context, result *model.PricingResult) error {
	a.logger.Infof("TEMU商品 %s 接受价格: %.2f", result.ProductName, result.SuggestPrice)

	// 这里应该调用TEMU的API接受价格
	// 由于需要具体的TEMU客户端，这里只是示例
	// temuClient.AcceptPrice(result.ProductID, result.SuggestPrice)

	return nil
}

// executeReject 执行拒绝决策
func (a *TemuAdapter) executeReject(ctx context.Context, result *model.PricingResult) error {
	a.logger.Infof("TEMU商品 %s 拒绝价格: %.2f", result.ProductName, result.SuggestPrice)

	// 这里应该调用TEMU的API拒绝价格或下架商品
	// temuClient.RejectPrice(result.ProductID)

	return nil
}

// executeReappeal 执行重新申诉决策
func (a *TemuAdapter) executeReappeal(ctx context.Context, result *model.PricingResult) error {
	a.logger.Infof("TEMU商品 %s 重新申诉，建议价格: %.2f", result.ProductName, result.AcceptablePrice)

	// 这里应该调用TEMU的API重新申诉价格
	// temuClient.ReappealPrice(result.ProductID, result.AcceptablePrice)

	return nil
}

// ConvertFromTemuSku 从TEMU SKU转换为通用核价上下文（简化版本）
func (a *TemuAdapter) ConvertFromTemuSku(sku interface{}, storeID int64, tenantID int64) *model.PricingContext {
	// 注意：这是简化版本，主要用于框架扩展
	// 原有的TEMU核价逻辑在增强服务中保持完全不变
	return &model.PricingContext{
		TenantID:     tenantID,
		StoreID:      storeID,
		ProductID:    "temu_product",
		SkuID:        "temu_sku",
		ProductName:  "TEMU商品",
		CurrentPrice: 0.0,
		SuggestPrice: 0.0,
		PlatformData: map[string]interface{}{
			"platform": "temu",
			"sku_data": sku,
		},
	}
}

// ConvertFromTemuSalesBoost 从TEMU销量提升数据转换为通用核价上下文（简化版本）
func (a *TemuAdapter) ConvertFromTemuSalesBoost(goods interface{}, sku interface{}, storeID int64, tenantID int64) *model.PricingContext {
	// 注意：这是简化版本，主要用于框架扩展
	// 原有的TEMU核价逻辑在增强服务中保持完全不变
	return &model.PricingContext{
		TenantID:     tenantID,
		StoreID:      storeID,
		ProductID:    "temu_sales_boost_product",
		SkuID:        "temu_sales_boost_sku",
		ProductName:  "TEMU销量提升商品",
		CurrentPrice: 0.0,
		SuggestPrice: 0.0,
		PlatformData: map[string]interface{}{
			"platform":    "temu",
			"sales_boost": true,
			"goods_data":  goods,
			"sku_data":    sku,
		},
	}
}
