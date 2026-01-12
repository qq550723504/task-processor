package services

import (
	"fmt"
	"task-processor/internal/platforms/temu/api/client"
	"task-processor/internal/platforms/temu/api/models"

	"github.com/sirupsen/logrus"
)

const (
	// 待核价列表接口
	pendingPriceListEndpoint = "/mms/marigold/price/v2/search_sales_boost"
	// 拒绝平台报价接口
	rejectPriceEndpoint = "/mms/marigold/sku/offline"
	// 重新报价接口
	reappealPriceEndpoint = "/mms/marigold/price/appeal/order/create"
	// 接受平台报价接口
	acceptPriceEndpoint = "/mms/marigold/price/goods/change"
)

// PricingAPI 定价API管理器
type PricingAPI struct {
	client client.APIClientInterface
	logger *logrus.Entry
}

// NewPricingAPI 创建新的定价API管理器
func NewPricingAPI(client client.APIClientInterface, logger *logrus.Entry) *PricingAPI {
	return &PricingAPI{
		client: client,
		logger: logger,
	}
}

// GetPendingPriceList 获取待核价列表
func (p *PricingAPI) GetPendingPriceList(pageNo, pageSize int) (*models.PendingPriceListResponse, error) {
	p.logger.Infof("获取待核价列表: pageNo=%d, pageSize=%d", pageNo, pageSize)

	req := &models.PendingPriceListRequest{
		PageSize: pageSize,
		PageNo:   pageNo,
		Scene:    "PRICING_HEALTH_SALES_BOOST", // 价格健康-销量提升场景
	}

	headers := client.GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["x-document-referer"] = "https://seller.temu.com/"

	request := map[string]interface{}{
		"method":  "POST",
		"url":     pendingPriceListEndpoint,
		"headers": headers,
		"body":    req,
	}

	var result models.PendingPriceListResponse
	if err := p.client.SendTEMURequest(request, &result); err != nil {
		p.logger.WithError(err).Error("获取待核价列表失败")
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

// RejectPrice 拒绝平台报价
func (p *PricingAPI) RejectPrice(goodsID string, skuIDs []string) (*models.RejectPriceResponse, error) {
	p.logger.Infof("拒绝平台报价: goodsID=%s, skuIDs=%v", goodsID, skuIDs)

	if goodsID == "" {
		return nil, fmt.Errorf("商品ID不能为空")
	}

	if len(skuIDs) == 0 {
		return nil, fmt.Errorf("SKU ID列表不能为空")
	}

	req := &models.RejectPriceRequest{
		GoodsID:         goodsID,
		SkuIDs:          skuIDs,
		OperationSource: 1005,
	}

	headers := client.GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["x-document-referer"] = "https://seller.temu.com/products.html"

	request := map[string]interface{}{
		"method":  "POST",
		"url":     rejectPriceEndpoint,
		"headers": headers,
		"body":    req,
	}

	var result models.RejectPriceResponse
	if err := p.client.SendTEMURequest(request, &result); err != nil {
		p.logger.WithError(err).Error("拒绝平台报价失败")
		return nil, fmt.Errorf("拒绝平台报价失败: %w", err)
	}

	if !result.Success {
		p.logger.Errorf("拒绝平台报价失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("拒绝平台报价失败: errorCode=%d", result.ErrorCode)
	}

	p.logger.Info("成功拒绝平台报价")
	return &result, nil
}

// ReappealPrice 重新报价
func (p *PricingAPI) ReappealPrice(goodsID string, skuInfoList []models.ReappealSkuInfo, appealSource int, appealReasons []string) (*models.ReappealPriceResponse, error) {
	p.logger.Infof("重新报价: goodsID=%s, SKU数量=%d", goodsID, len(skuInfoList))

	if goodsID == "" {
		return nil, fmt.Errorf("商品ID不能为空")
	}

	if len(skuInfoList) == 0 {
		return nil, fmt.Errorf("SKU信息列表不能为空")
	}

	req := &models.ReappealPriceRequest{
		GoodsID:              goodsID,
		AppealSource:         appealSource,
		MerchantAppealReason: appealReasons,
		SkuInfoList:          skuInfoList,
	}

	headers := client.GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["x-document-referer"] = "https://seller.temu.com/products.html"

	request := map[string]interface{}{
		"method":  "POST",
		"url":     reappealPriceEndpoint,
		"headers": headers,
		"body":    req,
	}

	var result models.ReappealPriceResponse
	if err := p.client.SendTEMURequest(request, &result); err != nil {
		p.logger.WithError(err).Error("重新报价失败")
		return nil, fmt.Errorf("重新报价失败: %w", err)
	}

	if !result.Success {
		p.logger.Errorf("重新报价失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("重新报价失败: errorCode=%d", result.ErrorCode)
	}

	p.logger.Info("成功提交重新报价")
	return &result, nil
}

// AcceptPrice 接受平台报价
func (p *PricingAPI) AcceptPrice(goodsID string, skuList []models.AcceptPriceSkuInfo, scene int) (*models.AcceptPriceResponse, error) {
	p.logger.Infof("接受平台报价: goodsID=%s, SKU数量=%d, scene=%d", goodsID, len(skuList), scene)

	if goodsID == "" {
		return nil, fmt.Errorf("商品ID不能为空")
	}

	if len(skuList) == 0 {
		return nil, fmt.Errorf("SKU列表不能为空")
	}

	req := &models.AcceptPriceRequest{
		Scene:   scene,
		GoodsID: goodsID,
		SkuList: skuList,
	}

	headers := client.GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["x-document-referer"] = "https://seller.temu.com/products.html"

	request := map[string]interface{}{
		"method":  "POST",
		"url":     acceptPriceEndpoint,
		"headers": headers,
		"body":    req,
	}

	var result models.AcceptPriceResponse
	if err := p.client.SendTEMURequest(request, &result); err != nil {
		p.logger.WithError(err).Error("接受平台报价失败")
		return nil, fmt.Errorf("接受平台报价失败: %w", err)
	}

	if !result.Success {
		p.logger.Errorf("接受平台报价失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("接受平台报价失败: errorCode=%d", result.ErrorCode)
	}

	p.logger.Info("成功接受平台报价")
	return &result, nil
}
