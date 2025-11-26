package temu

import (
	"fmt"
	"task-processor/common/management"
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

// GetPendingPriceList 获取待核价列表
func (c *APIClient) GetPendingPriceList(pageNo, pageSize int) (*PendingPriceListResponse, error) {
	c.logger.Infof("获取待核价列表: pageNo=%d, pageSize=%d", pageNo, pageSize)

	req := &PendingPriceListRequest{
		PageSize: pageSize,
		PageNo:   pageNo,
		Scene:    "PRICING_HEALTH_SALES_BOOST", // 价格健康-销量提升场景
	}

	headers := GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["x-document-referer"] = "https://seller.temu.com/"

	request := map[string]interface{}{
		"method":  "POST",
		"url":     pendingPriceListEndpoint,
		"headers": headers,
		"body":    req,
	}

	var result PendingPriceListResponse
	if err := c.SendTEMURequest(request, &result); err != nil {
		c.logger.WithError(err).Error("获取待核价列表失败")
		return nil, fmt.Errorf("获取待核价列表失败: %w", err)
	}

	if !result.Success {
		c.logger.Errorf("获取待核价列表失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("获取待核价列表失败: errorCode=%d", result.ErrorCode)
	}

	c.logger.Infof("成功获取待核价列表: 总数=%d, 当前页商品数=%d",
		result.Result.Total, len(result.Result.SalesBoostGoodsList))
	return &result, nil
}

// RejectPrice 拒绝平台报价
func (c *APIClient) RejectPrice(goodsID string, skuIDs []string) (*RejectPriceResponse, error) {
	c.logger.Infof("拒绝平台报价: goodsID=%s, skuIDs数量=%d", goodsID, len(skuIDs))

	if goodsID == "" {
		return nil, fmt.Errorf("商品ID不能为空")
	}

	if len(skuIDs) == 0 {
		return nil, fmt.Errorf("SKU ID列表不能为空")
	}

	req := &RejectPriceRequest{
		GoodsID:         goodsID,
		SkuIDs:          skuIDs,
		OperationSource: 1005,
	}

	headers := GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["x-document-referer"] = "https://seller.temu.com/products.html"

	request := map[string]interface{}{
		"method":  "POST",
		"url":     rejectPriceEndpoint,
		"headers": headers,
		"body":    req,
	}

	var result RejectPriceResponse
	if err := c.SendTEMURequest(request, &result); err != nil {
		c.logger.WithError(err).Error("拒绝平台报价失败")
		return nil, fmt.Errorf("拒绝平台报价失败: %w", err)
	}

	if !result.Success {
		c.logger.Errorf("拒绝平台报价失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("拒绝平台报价失败: errorCode=%d", result.ErrorCode)
	}

	c.logger.Info("成功拒绝平台报价")
	return &result, nil
}

// ReappealPrice 重新报价
func (c *APIClient) ReappealPrice(goodsID string, skuInfoList []ReappealSkuInfo, appealSource int, appealReasons []string) (*ReappealPriceResponse, error) {
	c.logger.Infof("重新报价: goodsID=%s, SKU数量=%d", goodsID, len(skuInfoList))

	if goodsID == "" {
		return nil, fmt.Errorf("商品ID不能为空")
	}

	if len(skuInfoList) == 0 {
		return nil, fmt.Errorf("SKU信息列表不能为空")
	}

	req := &ReappealPriceRequest{
		GoodsID:              goodsID,
		AppealSource:         appealSource,
		MerchantAppealReason: appealReasons,
		SkuInfoList:          skuInfoList,
	}

	headers := GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["x-document-referer"] = "https://seller.temu.com/products.html"

	request := map[string]interface{}{
		"method":  "POST",
		"url":     reappealPriceEndpoint,
		"headers": headers,
		"body":    req,
	}

	var result ReappealPriceResponse
	if err := c.SendTEMURequest(request, &result); err != nil {
		c.logger.WithError(err).Error("重新报价失败")
		return nil, fmt.Errorf("重新报价失败: %w", err)
	}

	if !result.Success {
		c.logger.Errorf("重新报价失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("重新报价失败: errorCode=%d", result.ErrorCode)
	}

	c.logger.Info("成功提交重新报价")
	return &result, nil
}

// AcceptPrice 接受平台报价
func (c *APIClient) AcceptPrice(goodsID string, skuList []AcceptPriceSkuInfo, scene int) (*AcceptPriceResponse, error) {
	c.logger.Infof("接受平台报价: goodsID=%s, SKU数量=%d, scene=%d", goodsID, len(skuList), scene)

	if goodsID == "" {
		return nil, fmt.Errorf("商品ID不能为空")
	}

	if len(skuList) == 0 {
		return nil, fmt.Errorf("SKU列表不能为空")
	}

	req := &AcceptPriceRequest{
		Scene:   scene,
		GoodsID: goodsID,
		SkuList: skuList,
	}

	headers := GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["x-document-referer"] = "https://seller.temu.com/products.html"

	request := map[string]interface{}{
		"method":  "POST",
		"url":     acceptPriceEndpoint,
		"headers": headers,
		"body":    req,
	}

	var result AcceptPriceResponse
	if err := c.SendTEMURequest(request, &result); err != nil {
		c.logger.WithError(err).Error("接受平台报价失败")
		return nil, fmt.Errorf("接受平台报价失败: %w", err)
	}

	if !result.Success {
		c.logger.Errorf("接受平台报价失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("接受平台报价失败: errorCode=%d", result.ErrorCode)
	}

	c.logger.Info("成功接受平台报价")
	return &result, nil
}

// AutoProcessPendingPricesWithRules 根据利润率规则智能处理待核价商品
func (c *APIClient) AutoProcessPendingPricesWithRules(managementClient *management.ClientManager) (*PricingStatistics, error) {
	c.logger.Info("开始智能核价处理")

	// 参数校验
	if managementClient == nil {
		return nil, fmt.Errorf("managementClient不能为空")
	}

	// 创建决策服务
	decisionService := NewPricingDecisionService(managementClient, c.tenantID, c.storeID)
	if decisionService == nil {
		return nil, fmt.Errorf("创建决策服务失败")
	}

	stats := &PricingStatistics{}
	pageNo := 1
	pageSize := 25

	for {
		// 获取待核价列表
		resp, err := c.GetPendingPriceList(pageNo, pageSize)
		if err != nil {
			return stats, fmt.Errorf("获取待核价列表失败: %w", err)
		}

		if resp == nil || len(resp.Result.SalesBoostGoodsList) == 0 {
			c.logger.Info("没有更多待核价商品")
			break
		}

		// 遍历商品列表
		for _, goods := range resp.Result.SalesBoostGoodsList {
			// 遍历每个商品的SKU列表
			for _, sku := range goods.SalesBoostSkuList {
				stats.TotalProcessed++

				// 做出决策
				decision, err := decisionService.MakeDecisionForSalesBoost(&goods, &sku, c.storeID)
				if err != nil {
					c.logger.WithError(err).Warnf("商品 %s SKU %s 决策失败",
						goods.SalesBoostGoodsBasicInfo.GoodsName, sku.SkuID)
					stats.FailCount++
					continue
				}

				if decision == nil {
					c.logger.Warnf("商品 %s SKU %s 决策结果为空",
						goods.SalesBoostGoodsBasicInfo.GoodsName, sku.SkuID)
					stats.FailCount++
					continue
				}

				// 执行决策
				if err := c.executeDecisionForSalesBoost(decision, &goods, &sku); err != nil {
					c.logger.WithError(err).Warnf("商品 %s SKU %s 执行决策失败: %s",
						goods.SalesBoostGoodsBasicInfo.GoodsName, sku.SkuID, decision.Action)
					stats.FailCount++
				} else {
					stats.SuccessCount++
					// 更新统计
					switch decision.Action {
					case DecisionAccept:
						stats.AcceptCount++
					case DecisionReject:
						stats.RejectCount++
					case DecisionReappeal:
						stats.ReappealCount++
					case DecisionSkip:
						stats.SkipCount++
					}
				}

				c.logger.Infof("商品 %s SKU %s 决策: %s, 原因: %s",
					goods.SalesBoostGoodsBasicInfo.GoodsName, sku.SkuID,
					decision.Action, decision.Reason)
			}
		}

		// 检查是否处理完所有商品
		if stats.TotalProcessed >= resp.Result.Total {
			break
		}

		pageNo++
	}

	c.logger.Infof("智能核价完成: 总数=%d, 接受=%d, 拒绝=%d, 重新报价=%d, 跳过=%d, 成功=%d, 失败=%d",
		stats.TotalProcessed, stats.AcceptCount, stats.RejectCount,
		stats.ReappealCount, stats.SkipCount, stats.SuccessCount, stats.FailCount)

	return stats, nil
}

// executeDecisionForSalesBoost 执行核价决策（新版本，适配销量提升场景）
func (c *APIClient) executeDecisionForSalesBoost(decision *PricingDecision, goods *SalesBoostGoods, sku *SalesBoostSku) error {
	switch decision.Action {
	case DecisionAccept:
		// 接受平台报价
		if sku.TargetSupplierPrice.Amount == "" || sku.TargetSupplierPrice.Currency == "" {
			return fmt.Errorf("目标价格信息不完整")
		}
		skuList := []AcceptPriceSkuInfo{
			{
				SkuID:                  sku.SkuID,
				Currency:               sku.TargetSupplierPrice.Currency,
				TargetSupplierPriceStr: sku.TargetSupplierPrice.Amount, // 使用平台推荐的目标价格
			},
		}
		_, err := c.AcceptPrice(goods.SalesBoostGoodsBasicInfo.GoodsID, skuList, 2)
		return err

	case DecisionReject:
		// 拒绝报价
		skuIDs := []string{sku.SkuID}
		_, err := c.RejectPrice(goods.SalesBoostGoodsBasicInfo.GoodsID, skuIDs)
		return err

	case DecisionReappeal:
		if sku.CurrentSupplierPrice.Amount == "" || sku.TargetSupplierPrice.Amount == "" || sku.CurrentSupplierPrice.Currency == "" {
			return fmt.Errorf("价格信息不完整")
		}
		skuInfoList := []ReappealSkuInfo{
			{
				SkuID:                       sku.SkuID,
				SupplierPriceStr:            sku.CurrentSupplierPrice.Amount,
				RecommendedSupplierPriceStr: sku.TargetSupplierPrice.Amount,
				TargetSupplierPriceStr:      fmt.Sprintf("%.2f", decision.AcceptablePrice),
				Currency:                    sku.CurrentSupplierPrice.Currency,
			},
		}
		// 使用 TEMU API 要求的申诉原因枚举值
		appealReasons := []string{"HIGH_COST"}
		_, err := c.ReappealPrice(goods.SalesBoostGoodsBasicInfo.GoodsID, skuInfoList, 100, appealReasons)
		return err

	case DecisionSkip:
		// 跳过，不做任何操作
		c.logger.Info("跳过")
		return nil

	default:
		return fmt.Errorf("未知的决策动作: %s", decision.Action)
	}
}

// parsePrice 解析价格字符串为浮点数
func parsePrice(priceStr string) float64 {
	if priceStr == "" {
		return 0
	}
	var price float64
	fmt.Sscanf(priceStr, "%f", &price)
	return price
}
