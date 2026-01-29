// Package services 提供TEMU库存管理服务
package services

import (
	"fmt"
	"task-processor/internal/platforms/temu/api/client"
	"task-processor/internal/platforms/temu/api/models"

	"github.com/sirupsen/logrus"
)

const (
	// 库存修改接口
	stockEditEndpoint = "/mms/marigold/stock/edit"
)

// InventoryService TEMU库存管理服务
type InventoryService struct {
	client client.APIClientInterface
	logger *logrus.Entry
}

// NewInventoryService 创建TEMU库存管理服务实例
func NewInventoryService(client client.APIClientInterface, logger *logrus.Entry) *InventoryService {
	return &InventoryService{
		client: client,
		logger: logger,
	}
}

// EditStock 修改商品库存
func (s *InventoryService) EditStock(goodsID string, skuStockChanges []models.SkuStockChange) (*models.StockEditResponse, error) {
	s.logger.Infof("修改商品库存: goodsID=%s, SKU数量=%d", goodsID, len(skuStockChanges))

	// 参数校验
	if goodsID == "" {
		return nil, fmt.Errorf("商品ID不能为空")
	}

	if len(skuStockChanges) == 0 {
		return nil, fmt.Errorf("SKU库存变更列表不能为空")
	}

	req := &models.StockEditRequest{
		GoodsID:            goodsID,
		SkuStockChangeList: skuStockChanges,
	}

	headers := client.GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["x-document-referer"] = "https://seller.temu.com/products.html"

	request := map[string]any{
		"method":  "POST",
		"url":     stockEditEndpoint,
		"headers": headers,
		"body":    req,
	}

	var result models.StockEditResponse
	if err := s.client.SendTEMURequest(request, &result); err != nil {
		s.logger.WithError(err).Error("修改库存失败")
		return nil, fmt.Errorf("修改库存失败: %w", err)
	}

	if !result.Success {
		s.logger.Errorf("修改库存失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("修改库存失败: errorCode=%d", result.ErrorCode)
	}

	s.logger.Infof("成功修改库存: goodsID=%s", goodsID)
	return &result, nil
}

// SetStock 设置SKU库存（便捷方法）
func (s *InventoryService) SetStock(goodsID, skuID string, stock, stockDiff int) error {
	skuStockChanges := []models.SkuStockChange{
		{
			SkuID:                 skuID,
			CurrentShippingMode:   1,
			CurrentStockAvailable: stock,
			StockDiff:             stockDiff, // 根据你提供的示例
		},
	}

	result, err := s.EditStock(goodsID, skuStockChanges)
	if err != nil {
		return err
	}

	if !result.Result.Result {
		return fmt.Errorf("设置库存失败")
	}

	return nil
}

// OfflineProduct 下架产品
func (s *InventoryService) OfflineProduct(goodsID string, skuIDs []string) (*models.OfflineProductResponse, error) {
	s.logger.Infof("下架产品: goodsID=%s, SKU数量=%d", goodsID, len(skuIDs))

	// 参数校验
	if goodsID == "" {
		return nil, fmt.Errorf("商品ID不能为空")
	}

	if len(skuIDs) == 0 {
		return nil, fmt.Errorf("SKU ID列表不能为空")
	}

	req := &models.OfflineProductRequest{
		GoodsID: goodsID,
		SkuIDs:  skuIDs,
	}

	headers := client.GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["x-document-referer"] = "https://seller.temu.com/products.html"

	request := map[string]any{
		"method":  "POST",
		"url":     "/mms/marigold/sku/offline",
		"headers": headers,
		"body":    req,
	}

	var result models.OfflineProductResponse
	if err := s.client.SendTEMURequest(request, &result); err != nil {
		s.logger.WithError(err).Error("下架产品失败")
		return nil, fmt.Errorf("下架产品失败: %w", err)
	}

	if !result.Success {
		s.logger.Errorf("下架产品失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("下架产品失败: errorCode=%d", result.ErrorCode)
	}

	s.logger.Infof("成功下架产品: goodsID=%s", goodsID)
	return &result, nil
}

// OnlineProduct 上架产品
func (s *InventoryService) OnlineProduct(goodsID string, skuIDs []string) (*models.OnlineProductResponse, error) {
	s.logger.Infof("上架产品: goodsID=%s, SKU数量=%d", goodsID, len(skuIDs))

	// 参数校验
	if goodsID == "" {
		return nil, fmt.Errorf("商品ID不能为空")
	}

	if len(skuIDs) == 0 {
		return nil, fmt.Errorf("SKU ID列表不能为空")
	}

	req := &models.OnlineProductRequest{
		GoodsID: goodsID,
		SkuIDs:  skuIDs,
	}

	headers := client.GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["x-document-referer"] = "https://seller.temu.com/products.html"

	request := map[string]any{
		"method":  "POST",
		"url":     "/mms/marigold/sku/online",
		"headers": headers,
		"body":    req,
	}

	var result models.OnlineProductResponse
	if err := s.client.SendTEMURequest(request, &result); err != nil {
		s.logger.WithError(err).Error("上架产品失败")
		return nil, fmt.Errorf("上架产品失败: %w", err)
	}

	if !result.Success {
		s.logger.Errorf("上架产品失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("上架产品失败: errorCode=%d", result.ErrorCode)
	}

	// 检查操作结果，如果失败记录具体原因
	if !result.Result.Result && result.Result.Msg != "" {
		s.logger.Warnf("上架产品部分失败: goodsID=%s, 原因=%s", goodsID, result.Result.Msg)
	} else {
		s.logger.Infof("成功上架产品: goodsID=%s", goodsID)
	}

	return &result, nil
}
