package handlers

import (
	"context"
	"fmt"
	"task-processor/platforms/amazon"
	"task-processor/platforms/amazon/api"

	"github.com/sirupsen/logrus"
)

// PricingSetupHandler 价格设置处理器
type PricingSetupHandler struct {
	apiClient *api.Client
}

// NewPricingSetupHandler 创建价格设置处理器
func NewPricingSetupHandler(apiClient *api.Client) *PricingSetupHandler {
	return &PricingSetupHandler{
		apiClient: apiClient,
	}
}

// Name 返回处理器名称
func (h *PricingSetupHandler) Name() string {
	return "设置产品价格"
}

// Handle 处理逻辑
func (h *PricingSetupHandler) Handle(ctx *amazon.TaskContext) error {
	logrus.Info("[PricingSetup] 开始设置产品价格")

	// 1. 获取SKU
	sku, exists := ctx.GetData("listing_sku")
	if !exists {
		return fmt.Errorf("SKU不存在")
	}

	skuStr, ok := sku.(string)
	if !ok {
		return fmt.Errorf("SKU格式错误")
	}

	// 2. 从原始数据中提取价格
	price := h.extractPrice(ctx)
	if price <= 0 {
		return fmt.Errorf("无效的价格: %.2f", price)
	}

	// 3. 构建价格请求
	req := &api.PriceRequest{
		SKU:      skuStr,
		Price:    price,
		Currency: "USD", // 默认美元
	}

	// 4. 调用API设置价格
	apiCtx := context.Background()
	resp, err := h.apiClient.UpdatePrice(apiCtx, req)
	if err != nil {
		return fmt.Errorf("设置价格失败: %w", err)
	}

	// 5. 保存结果
	ctx.SetData("pricing_amount", resp.Price)
	ctx.SetData("pricing_currency", resp.Currency)
	ctx.SetData("pricing_status", resp.Status)

	logrus.Infof("[PricingSetup] 价格设置完成: SKU=%s, Price=%.2f %s",
		skuStr, resp.Price, resp.Currency)
	return nil
}

// extractPrice 从产品数据中提取价格
func (h *PricingSetupHandler) extractPrice(ctx *amazon.TaskContext) float64 {
	// 从原始产品数据中获取
	rawData, exists := ctx.GetData("raw_product_data")
	if !exists {
		logrus.Warn("[PricingSetup] 未找到原始产品数据")
		return 0
	}

	productData, ok := rawData.(map[string]interface{})
	if !ok {
		logrus.Warn("[PricingSetup] 产品数据格式错误")
		return 0
	}

	// 尝试从不同字段获取价格
	priceFields := []string{
		"price",
		"salePrice",
		"retailPrice",
		"listPrice",
		"unitPrice",
	}

	for _, field := range priceFields {
		if val, ok := productData[field]; ok {
			price := h.parsePrice(val)
			if price > 0 {
				// 应用价格策略（例如：加价30%）
				return h.applyPricingStrategy(price)
			}
		}
	}

	logrus.Warn("[PricingSetup] 未找到有效的价格字段")
	return 0
}

// parsePrice 解析价格值
func (h *PricingSetupHandler) parsePrice(val interface{}) float64 {
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
func (h *PricingSetupHandler) applyPricingStrategy(originalPrice float64) float64 {
	// 简单的加价策略：
	// 1. 人民币转美元（假设汇率1:0.14）
	priceUSD := originalPrice * 0.14

	// 2. 加价30%作为利润
	priceWithProfit := priceUSD * 1.30

	// 3. 四舍五入到2位小数
	return float64(int(priceWithProfit*100)) / 100
}
