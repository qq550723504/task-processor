// package pipeline 提供价格处理器实现
package pipeline

import (
	"context"
	"fmt"
	"task-processor/internal/amazon/api"
	"task-processor/internal/amazon/model"
)

// PricingHandler 价格处理器
type PricingHandler struct {
	*BaseHandler
}

// NewPricingHandler 创建价格处理器
func NewPricingHandler(services *model.Services) *PricingHandler {
	return &PricingHandler{
		BaseHandler: NewBaseHandler("设置产品价格"),
	}
}

// Handle 执行价格设置
func (h *PricingHandler) Handle(ctx context.Context, taskContext *model.TaskContext) error {
	h.logger.Info("开始设置产品价格")

	// 获取SKU（从Results中获取）
	sku, exists := taskContext.GetResult("listing_sku")
	if !exists {
		return fmt.Errorf("SKU不存在")
	}

	skuStr, ok := sku.(string)
	if !ok {
		return fmt.Errorf("SKU格式错误")
	}

	// 计算价格
	price := h.calculatePrice(taskContext.Data)
	if price <= 0 {
		return fmt.Errorf("无效的价格: %.2f", price)
	}

	// 模拟价格设置成功
	mockResponse := &api.PriceResponse{
		SKU:      skuStr,
		Price:    price,
		Currency: "USD",
		Status:   "ACTIVE",
	}

	// 保存结果
	taskContext.SetResult("pricing_amount", mockResponse.Price)
	taskContext.SetResult("pricing_currency", mockResponse.Currency)
	taskContext.SetResult("pricing_status", mockResponse.Status)

	h.logger.Infof("价格设置完成: SKU=%s, Price=%.2f %s", skuStr, mockResponse.Price, mockResponse.Currency)
	return nil
}

// calculatePrice 计算价格
func (h *PricingHandler) calculatePrice(data map[string]any) float64 {
	// 从原始数据中提取价格
	rawPrice := h.extractOriginalPrice(data)
	if rawPrice <= 0 {
		// 使用默认价格
		return 19.99
	}

	// 应用价格策略
	return h.applyPricingStrategy(rawPrice)
}

// extractOriginalPrice 提取原始价格
func (h *PricingHandler) extractOriginalPrice(data map[string]any) float64 {
	// 尝试从不同字段获取价格
	priceFields := []string{
		"original_price",
		"price",
		"sale_price",
		"list_price",
	}

	for _, field := range priceFields {
		if val, exists := data[field]; exists {
			if price := h.parsePrice(val); price > 0 {
				return price
			}
		}
	}

	return 0
}

// parsePrice 解析价格值
func (h *PricingHandler) parsePrice(val any) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		var price float64
		if _, err := fmt.Sscanf(v, "%f", &price); err == nil {
			return price
		}
	}
	return 0
}

// applyPricingStrategy 应用价格策略
func (h *PricingHandler) applyPricingStrategy(originalPrice float64) float64 {
	// 简单的加价策略：
	// 1. 人民币转美元（假设汇率1:0.14）
	priceUSD := originalPrice * 0.14

	// 2. 加价30%作为利润
	priceWithProfit := priceUSD * 1.30

	// 3. 四舍五入到2位小数
	return float64(int(priceWithProfit*100)) / 100
}

