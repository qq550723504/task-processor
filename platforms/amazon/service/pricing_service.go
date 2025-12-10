package service

import (
	"context"
	"fmt"
	"task-processor/platforms/amazon/api"

	"github.com/sirupsen/logrus"
)

// PricingService 价格业务服务
type PricingService struct {
	apiClient *api.Client
	logger    *logrus.Entry
}

// NewPricingService 创建价格服务
func NewPricingService(apiClient *api.Client) *PricingService {
	return &PricingService{
		apiClient: apiClient,
		logger: logrus.WithFields(logrus.Fields{
			"service": "PricingService",
		}),
	}
}

// UpdatePrice 更新价格
func (s *PricingService) UpdatePrice(ctx context.Context, sku string, price float64, currency string) error {
	s.logger.WithFields(logrus.Fields{
		"sku":      sku,
		"price":    price,
		"currency": currency,
	}).Info("更新价格")

	if price <= 0 {
		return fmt.Errorf("价格必须大于0")
	}

	if currency == "" {
		currency = "USD"
	}

	req := &api.PriceRequest{
		SKU:      sku,
		Price:    price,
		Currency: currency,
	}

	_, err := s.apiClient.UpdatePrice(ctx, req)
	return err
}

// GetPrice 获取价格
func (s *PricingService) GetPrice(ctx context.Context, sku string) (float64, string, error) {
	s.logger.WithFields(logrus.Fields{
		"sku": sku,
	}).Info("获取价格")

	resp, err := s.apiClient.GetPrice(ctx, sku)
	if err != nil {
		return 0, "", err
	}

	return resp.Price, resp.Currency, nil
}

// CalculatePriceWithProfit 根据成本和利润率计算售价
func (s *PricingService) CalculatePriceWithProfit(cost float64, profitRate float64) float64 {
	if cost <= 0 || profitRate < 0 {
		return 0
	}

	return cost * (1 + profitRate/100)
}
