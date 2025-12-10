package handlers

import (
	"fmt"
	"task-processor/platforms/amazon"
	"task-processor/platforms/amazon/api"

	"github.com/sirupsen/logrus"
)

// PricingHandler 价格处理器
type PricingHandler struct {
	apiClient *api.Client
}

// NewPricingHandler 创建价格处理器
func NewPricingHandler(apiClient *api.Client) *PricingHandler {
	return &PricingHandler{
		apiClient: apiClient,
	}
}

// Name 返回处理器名称
func (h *PricingHandler) Name() string {
	return "设置价格"
}

// Handle 处理逻辑
func (h *PricingHandler) Handle(ctx *amazon.TaskContext) error {
	logrus.Info("开始设置价格")

	// 获取SKU
	sku := ctx.Task.ProductID

	// 构建价格请求
	req := &api.PriceRequest{
		SKU:      sku,
		Price:    h.calculatePrice(ctx),
		Currency: "USD",
	}

	// 更新价格
	resp, err := h.apiClient.UpdatePrice(ctx.Context, req)
	if err != nil {
		return fmt.Errorf("更新价格失败: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"sku":      resp.SKU,
		"price":    resp.Price,
		"currency": resp.Currency,
	}).Info("价格设置成功")

	return nil
}

// calculatePrice 计算价格
func (h *PricingHandler) calculatePrice(ctx *amazon.TaskContext) float64 {
	// TODO: 从产品数据中获取价格，应用利润率等
	return 19.99
}
