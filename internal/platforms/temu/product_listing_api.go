package temu

import (
	"fmt"
)

const (
	// 重新上架接口
	relistProductEndpoint = "/mms/marigold/sku/online"
	// 下架产品接口
	delistProductEndpoint = "/mms/marigold/sku/offline"
)

// RelistProduct 重新上架产品
func (c *APIClient) RelistProduct(goodsID string, skuIDs []string) (*RelistProductResponse, error) {
	c.logger.Infof("重新上架产品: goodsID=%s, skuIDs数量=%d, skuIDs=%v", goodsID, len(skuIDs), skuIDs)

	// 参数校验
	if goodsID == "" {
		return nil, fmt.Errorf("商品ID不能为空")
	}

	if len(skuIDs) == 0 {
		return nil, fmt.Errorf("SKU ID列表不能为空")
	}

	req := &RelistProductRequest{
		GoodsID: goodsID,
		SkuIDs:  skuIDs,
	}

	headers := GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["x-document-referer"] = "https://seller.temu.com/products.html"

	request := map[string]any{
		"method":  "POST",
		"url":     relistProductEndpoint,
		"headers": headers,
		"body":    req,
	}

	var result RelistProductResponse
	if err := c.SendTEMURequest(request, &result); err != nil {
		c.logger.WithError(err).Error("重新上架产品失败")
		return nil, fmt.Errorf("重新上架产品失败: %w", err)
	}

	// 特别检查错误码
	if result.ErrorCode != 1000000 {
		c.logger.Errorf("重新上架产品错误码异常: errorCode=%d, 通常1000000表示成功", result.ErrorCode)
	}

	if !result.Success {
		c.logger.Errorf("重新上架产品失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("重新上架产品失败: errorCode=%d", result.ErrorCode)
	}

	if result.Result.Result {
		c.logger.Info("产品重新上架成功")
	} else {
		c.logger.Warnf("产品重新上架请求成功但结果为false，可能存在限制条件 - goodsID=%s, skuCount=%d", goodsID, len(skuIDs))

		// 尝试分析可能的原因
		if len(skuIDs) > 1 {
			c.logger.Warnf("多SKU商品上架失败，可能需要逐个SKU处理")
		}

		// 记录详细的失败信息以便调试
		c.logger.Warnf("上架失败详情: 请求体=%+v", req)
	}

	return &result, nil
}

// DelistProduct 下架产品
func (c *APIClient) DelistProduct(goodsID string, skuIDs []string, operationSource int) (*DelistProductResponse, error) {
	c.logger.Infof("下架产品: goodsID=%s, skuIDs数量=%d, operationSource=%d", goodsID, len(skuIDs), operationSource)

	// 参数校验
	if goodsID == "" {
		return nil, fmt.Errorf("商品ID不能为空")
	}

	if len(skuIDs) == 0 {
		return nil, fmt.Errorf("SKU ID列表不能为空")
	}

	req := &DelistProductRequest{
		GoodsID:         goodsID,
		SkuIDs:          skuIDs,
		OperationSource: operationSource, // 操作来源，如1005表示价格健康页面
	}

	headers := GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["x-document-referer"] = "https://seller.temu.com/products.html"

	request := map[string]any{
		"method":  "POST",
		"url":     delistProductEndpoint,
		"headers": headers,
		"body":    req,
	}

	var result DelistProductResponse
	if err := c.SendTEMURequest(request, &result); err != nil {
		c.logger.WithError(err).Error("下架产品失败")
		return nil, fmt.Errorf("下架产品失败: %w", err)
	}

	if !result.Success {
		c.logger.Errorf("下架产品失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("下架产品失败: errorCode=%d", result.ErrorCode)
	}

	c.logger.Info("产品下架成功")
	return &result, nil
}

// BatchRelistProducts 批量重新上架产品
func (c *APIClient) BatchRelistProducts(products []ProductListingInfo) (*BatchListingResult, error) {
	c.logger.Infof("批量重新上架产品: 数量=%d", len(products))

	if len(products) == 0 {
		return nil, fmt.Errorf("产品列表不能为空")
	}

	result := &BatchListingResult{
		TotalCount:   len(products),
		SuccessCount: 0,
		FailCount:    0,
		Results:      make([]ListingOperationResult, 0, len(products)),
	}

	for _, product := range products {
		opResult := ListingOperationResult{
			GoodsID: product.GoodsID,
			SkuIDs:  product.SkuIDs,
		}

		resp, err := c.RelistProduct(product.GoodsID, product.SkuIDs)
		if err != nil {
			opResult.Success = false
			opResult.Error = err.Error()
			result.FailCount++
			c.logger.WithError(err).Warnf("重新上架产品失败: goodsID=%s", product.GoodsID)
		} else if resp != nil && resp.Result.Result {
			opResult.Success = true
			result.SuccessCount++
			c.logger.Infof("重新上架产品成功: goodsID=%s", product.GoodsID)
		} else {
			opResult.Success = false
			opResult.Error = "上架请求成功但结果为false"
			result.FailCount++
			c.logger.Warnf("重新上架产品结果为false: goodsID=%s", product.GoodsID)
		}

		result.Results = append(result.Results, opResult)
	}

	c.logger.Infof("批量重新上架完成: 总数=%d, 成功=%d, 失败=%d",
		result.TotalCount, result.SuccessCount, result.FailCount)

	return result, nil
}

// BatchDelistProducts 批量下架产品
func (c *APIClient) BatchDelistProducts(products []ProductListingInfo, operationSource int) (*BatchListingResult, error) {
	c.logger.Infof("批量下架产品: 数量=%d, operationSource=%d", len(products), operationSource)

	if len(products) == 0 {
		return nil, fmt.Errorf("产品列表不能为空")
	}

	result := &BatchListingResult{
		TotalCount:   len(products),
		SuccessCount: 0,
		FailCount:    0,
		Results:      make([]ListingOperationResult, 0, len(products)),
	}

	for _, product := range products {
		opResult := ListingOperationResult{
			GoodsID: product.GoodsID,
			SkuIDs:  product.SkuIDs,
		}

		resp, err := c.DelistProduct(product.GoodsID, product.SkuIDs, operationSource)
		if err != nil {
			opResult.Success = false
			opResult.Error = err.Error()
			result.FailCount++
			c.logger.WithError(err).Warnf("下架产品失败: goodsID=%s", product.GoodsID)
		} else if resp != nil {
			opResult.Success = true
			result.SuccessCount++
			c.logger.Infof("下架产品成功: goodsID=%s", product.GoodsID)
		} else {
			opResult.Success = false
			opResult.Error = "下架响应为空"
			result.FailCount++
			c.logger.Warnf("下架产品响应为空: goodsID=%s", product.GoodsID)
		}

		result.Results = append(result.Results, opResult)
	}

	c.logger.Infof("批量下架完成: 总数=%d, 成功=%d, 失败=%d",
		result.TotalCount, result.SuccessCount, result.FailCount)

	return result, nil
}

// RelistOfflineProductsWithConditions 根据条件重新上架已下架产品
func (c *APIClient) RelistOfflineProductsWithConditions(conditions *RelistConditions) (*BatchListingResult, error) {
	c.logger.Info("根据条件重新上架已下架产品")

	// 获取所有已下架产品
	offlineProducts, err := c.GetAllOfflineProducts()
	if err != nil {
		return nil, fmt.Errorf("获取已下架产品失败: %w", err)
	}

	// 根据条件筛选产品
	var filteredProducts []ProductListingInfo
	for _, product := range offlineProducts {
		if c.shouldRelistProduct(&product, conditions) {
			filteredProducts = append(filteredProducts, ProductListingInfo{
				GoodsID: product.GoodsID,
				SkuIDs:  []string{product.SkuID},
			})
		}
	}

	c.logger.Infof("筛选出符合条件的产品数量: %d", len(filteredProducts))

	if len(filteredProducts) == 0 {
		return &BatchListingResult{
			TotalCount:   0,
			SuccessCount: 0,
			FailCount:    0,
			Results:      []ListingOperationResult{},
		}, nil
	}

	// 批量重新上架
	return c.BatchRelistProducts(filteredProducts)
}

// RelistAllOfflineProducts 获取所有已下架产品并逐个全部上架
func (c *APIClient) RelistAllOfflineProducts() (*RelistAllResult, error) {
	c.logger.Info("开始获取所有已下架产品并逐个上架")

	// 获取所有已下架产品
	offlineProducts, err := c.GetAllOfflineProducts()
	if err != nil {
		return nil, fmt.Errorf("获取已下架产品失败: %w", err)
	}

	if len(offlineProducts) == 0 {
		c.logger.Info("没有已下架的产品")
		return &RelistAllResult{
			TotalOfflineCount: 0,
			ProcessedCount:    0,
			SuccessCount:      0,
			FailCount:         0,
			SkippedCount:      0,
			Results:           []RelistDetailResult{},
		}, nil
	}

	c.logger.Infof("找到 %d 个已下架产品，开始逐个上架", len(offlineProducts))

	result := &RelistAllResult{
		TotalOfflineCount: len(offlineProducts),
		ProcessedCount:    0,
		SuccessCount:      0,
		FailCount:         0,
		SkippedCount:      0,
		Results:           make([]RelistDetailResult, 0, len(offlineProducts)),
	}

	// 按商品ID分组，因为同一个商品可能有多个SKU
	goodsSkuMap := make(map[string][]string)
	productInfoMap := make(map[string]*OfflineProductItem)

	for _, product := range offlineProducts {
		if _, exists := goodsSkuMap[product.GoodsID]; !exists {
			goodsSkuMap[product.GoodsID] = []string{}
			productInfoMap[product.GoodsID] = &product
		}
		goodsSkuMap[product.GoodsID] = append(goodsSkuMap[product.GoodsID], product.SkuID)
	}

	c.logger.Infof("共有 %d 个不同的商品需要上架", len(goodsSkuMap))

	// 逐个商品进行上架
	for goodsID, skuIDs := range goodsSkuMap {
		result.ProcessedCount++
		productInfo := productInfoMap[goodsID]

		detailResult := RelistDetailResult{
			GoodsID:   goodsID,
			GoodsName: productInfo.GoodsName,
			SkuIDs:    skuIDs,
			SkuCount:  len(skuIDs),
		}

		c.logger.Infof("正在上架商品 [%d/%d]: %s (ID: %s, SKU数量: %d)",
			result.ProcessedCount, len(goodsSkuMap), productInfo.GoodsName, goodsID, len(skuIDs))

		// 检查是否应该跳过此商品
		if c.shouldSkipProduct(productInfo) {
			detailResult.Success = false
			detailResult.Skipped = true
			detailResult.Error = c.getSkipReason(productInfo)
			result.SkippedCount++
			c.logger.Warnf("跳过商品 %s: %s", productInfo.GoodsName, detailResult.Error)
		} else {
			// 尝试上架
			resp, err := c.RelistProduct(goodsID, skuIDs)
			if err != nil {
				detailResult.Success = false
				detailResult.Error = err.Error()
				result.FailCount++
				c.logger.WithError(err).Errorf("上架商品失败: %s", productInfo.GoodsName)
			} else if resp != nil && resp.Result.Result {
				detailResult.Success = true
				result.SuccessCount++
				c.logger.Infof("✓ 上架商品成功: %s", productInfo.GoodsName)
			} else {
				detailResult.Success = false
				detailResult.Error = "上架请求成功但结果为false，可能存在平台限制"
				result.FailCount++
				c.logger.Warnf("上架商品结果为false: %s", productInfo.GoodsName)
			}
		}

		result.Results = append(result.Results, detailResult)

		// 添加延迟避免请求过于频繁
		if result.ProcessedCount < len(goodsSkuMap) {
			c.logger.Debug("等待1秒后处理下一个商品...")
			// 这里可以添加 time.Sleep(1 * time.Second) 但为了避免阻塞，暂时注释
		}
	}

	c.logger.Infof("所有已下架产品上架完成: 总下架数=%d, 处理数=%d, 成功=%d, 失败=%d, 跳过=%d",
		result.TotalOfflineCount, result.ProcessedCount, result.SuccessCount, result.FailCount, result.SkippedCount)

	return result, nil
}

// shouldSkipProduct 判断是否应该跳过某个产品
func (c *APIClient) shouldSkipProduct(product *OfflineProductItem) bool {
	// 检查是否需要整改
	if product.CategoryRectificationInfo.NeedRectification {
		return true
	}

	// 检查是否被严重惩罚
	if product.PunishTags > 1 {
		return true
	}

	// 检查锁定状态
	if !product.LockInfo.AllowEditPersonalizationInfo {
		return true
	}

	return false
}

// getSkipReason 获取跳过原因
func (c *APIClient) getSkipReason(product *OfflineProductItem) string {
	if product.CategoryRectificationInfo.NeedRectification {
		return "商品需要分类整改"
	}

	if product.PunishTags > 1 {
		return "商品被严重惩罚"
	}

	if !product.LockInfo.AllowEditPersonalizationInfo {
		return "商品被锁定，不允许编辑"
	}

	return "未知原因"
}

// shouldRelistProduct 判断是否应该重新上架产品
func (c *APIClient) shouldRelistProduct(product *OfflineProductItem, conditions *RelistConditions) bool {
	if conditions == nil {
		return true // 没有条件限制，全部重新上架
	}

	// 检查是否有库存
	if conditions.RequireStock && product.Stock <= 0 {
		return false
	}

	// 检查是否需要整改
	if conditions.ExcludeNeedRectification && product.CategoryRectificationInfo.NeedRectification {
		return false
	}

	// 检查是否被惩罚
	if conditions.ExcludePunished && product.PunishTags > 0 {
		return false
	}

	// 检查分类
	if len(conditions.IncludeCategories) > 0 {
		found := false
		for _, category := range conditions.IncludeCategories {
			for _, productCategory := range product.CatNameList {
				if category == productCategory {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查排除分类
	if len(conditions.ExcludeCategories) > 0 {
		for _, category := range conditions.ExcludeCategories {
			for _, productCategory := range product.CatNameList {
				if category == productCategory {
					return false
				}
			}
		}
	}

	return true
}
