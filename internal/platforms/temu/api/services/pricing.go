// Package services 提供TEMU平台定价相关的业务服务
package services

import (
	"fmt"
	"task-processor/internal/platforms/temu/api/client"
	"task-processor/internal/platforms/temu/api/models"

	"github.com/sirupsen/logrus"
)

// PricingService 定价服务 - 处理所有定价相关的业务逻辑
type PricingService struct {
	client client.APIClientInterface
	logger *logrus.Entry
}

// NewPricingService 创建新的定价服务
func NewPricingService(client client.APIClientInterface, logger *logrus.Entry) *PricingService {
	return &PricingService{
		client: client,
		logger: logger,
	}
}

// getPendingPriceList 获取待核价列表
func (p *PricingService) getPendingPriceList(pageNo, pageSize int) (*models.PendingPriceListResponse, error) {
	p.logger.Infof("获取待核价列表: pageNo=%d, pageSize=%d", pageNo, pageSize)

	req := &models.PendingPriceListRequest{
		PageSize: pageSize,
		PageNo:   pageNo,
		Scene:    "PRICING_HEALTH_SALES_BOOST", // 价格健康-销量提升场景
	}

	headers := client.GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["x-document-referer"] = "https://seller.temu.com/"

	request := map[string]any{
		"method":  "POST",
		"url":     "/mms/marigold/price/v2/search_sales_boost",
		"headers": headers,
		"body":    req,
	}

	p.logger.Infof("发送API请求: %s", request["url"])

	var result models.PendingPriceListResponse
	if err := p.client.SendTEMURequest(request, &result); err != nil {
		p.logger.WithError(err).Error("获取待核价列表失败")

		// 检查是否是超时错误
		if isTimeoutError(err) {
			p.logger.Error("API调用超时，建议检查网络连接或增加超时时间")
			return nil, fmt.Errorf("API调用超时: %w", err)
		}

		return nil, fmt.Errorf("获取待核价列表失败: %w", err)
	}

	if !result.Success {
		p.logger.Errorf("获取待核价列表失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("获取待核价列表失败: errorCode=%d", result.ErrorCode)
	}

	p.logger.Infof("成功获取待核价列表: 总数=%d, 当前页商品数=%d",
		result.Result.Total, len(result.Result.SalesBoostGoodsList))

	return &result, nil
}

// isTimeoutError 检查是否是超时错误
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "timeout") ||
		contains(errStr, "deadline exceeded") ||
		contains(errStr, "context deadline exceeded") ||
		contains(errStr, "Client.Timeout exceeded")
}

// contains 检查字符串是否包含子字符串（不区分大小写）
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsInMiddle(s, substr))))
}

// containsInMiddle 检查字符串中间是否包含子字符串
func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// acceptPrice 接受平台报价
func (p *PricingService) acceptPrice(goodsID string, sku *models.SalesBoostSku) error {
	p.logger.Infof("接受平台报价: goodsID=%s, skuID=%s", goodsID, sku.SkuID)

	skuList := []models.AcceptPriceSkuInfo{
		{
			SkuID:                  sku.SkuID,
			Currency:               sku.TargetSupplierPrice.Currency,
			TargetSupplierPriceStr: sku.TargetSupplierPrice.Amount,
		},
	}

	req := &models.AcceptPriceRequest{
		Scene:   2,
		GoodsID: goodsID,
		SkuList: skuList,
	}

	headers := client.GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["x-document-referer"] = "https://seller.temu.com/products.html"

	request := map[string]any{
		"method":  "POST",
		"url":     "/mms/marigold/price/goods/change",
		"headers": headers,
		"body":    req,
	}

	var result models.AcceptPriceResponse
	if err := p.client.SendTEMURequest(request, &result); err != nil {
		p.logger.WithError(err).Error("接受平台报价失败")
		return fmt.Errorf("接受平台报价失败: %w", err)
	}

	if !result.Success {
		p.logger.Errorf("接受平台报价失败: errorCode=%d", result.ErrorCode)
		return fmt.Errorf("接受平台报价失败: errorCode=%d", result.ErrorCode)
	}

	p.logger.Info("成功接受平台报价")
	return nil
}

// rejectPrice 拒绝平台报价
func (p *PricingService) rejectPrice(goodsID string, skuIDs []string) error {
	p.logger.Infof("拒绝平台报价: goodsID=%s, skuIDs=%v", goodsID, skuIDs)

	req := &models.RejectPriceRequest{
		GoodsID:         goodsID,
		SkuIDs:          skuIDs,
		OperationSource: 1005,
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

	var result models.RejectPriceResponse
	if err := p.client.SendTEMURequest(request, &result); err != nil {
		p.logger.WithError(err).Error("拒绝平台报价失败")
		return fmt.Errorf("拒绝平台报价失败: %w", err)
	}

	if !result.Success {
		p.logger.Errorf("拒绝平台报价失败: errorCode=%d", result.ErrorCode)
		return fmt.Errorf("拒绝平台报价失败: errorCode=%d", result.ErrorCode)
	}

	p.logger.Info("成功拒绝平台报价")
	return nil
}

// GetPendingPriceList 获取待核价列表（公开方法）
func (p *PricingService) GetPendingPriceList(pageNo, pageSize int) (*models.PendingPriceListResponse, error) {
	return p.getPendingPriceList(pageNo, pageSize)
}

// AcceptPrice 接受平台报价（简化接口）
func (p *PricingService) AcceptPrice(goodsID string, sku *models.SalesBoostSku) error {
	return p.acceptPrice(goodsID, sku)
}

// RejectPrice 拒绝平台报价（简化接口）
func (p *PricingService) RejectPrice(goodsID string, skuIDs []string) error {
	return p.rejectPrice(goodsID, skuIDs)
}
