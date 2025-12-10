package service

import (
	"context"
	"fmt"
	"task-processor/platforms/amazon/api"

	"github.com/sirupsen/logrus"
)

// InventoryService 库存业务服务
type InventoryService struct {
	apiClient *api.Client
	logger    *logrus.Entry
}

// NewInventoryService 创建库存服务
func NewInventoryService(apiClient *api.Client) *InventoryService {
	return &InventoryService{
		apiClient: apiClient,
		logger: logrus.WithFields(logrus.Fields{
			"service": "InventoryService",
		}),
	}
}

// UpdateInventory 更新库存
func (s *InventoryService) UpdateInventory(ctx context.Context, sku string, quantity int) error {
	s.logger.WithFields(logrus.Fields{
		"sku":      sku,
		"quantity": quantity,
	}).Info("更新库存")

	if quantity < 0 {
		return fmt.Errorf("库存数量不能为负数")
	}

	req := &api.InventoryRequest{
		SKU:      sku,
		Quantity: quantity,
	}

	_, err := s.apiClient.UpdateInventory(ctx, req)
	return err
}

// GetInventory 获取库存
func (s *InventoryService) GetInventory(ctx context.Context, sku string) (int, error) {
	s.logger.WithFields(logrus.Fields{
		"sku": sku,
	}).Info("获取库存")

	resp, err := s.apiClient.GetInventory(ctx, sku)
	if err != nil {
		return 0, err
	}

	return resp.Quantity, nil
}

// BatchUpdateInventory 批量更新库存
func (s *InventoryService) BatchUpdateInventory(ctx context.Context, items map[string]int) error {
	s.logger.WithFields(logrus.Fields{
		"count": len(items),
	}).Info("批量更新库存")

	requests := make([]*api.InventoryRequest, 0, len(items))
	for sku, quantity := range items {
		requests = append(requests, &api.InventoryRequest{
			SKU:      sku,
			Quantity: quantity,
		})
	}

	_, err := s.apiClient.BatchUpdateInventory(ctx, requests)
	return err
}
