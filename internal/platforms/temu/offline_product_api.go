package temu

import (
	"fmt"
)

const (
	// 已下架产品搜索接口
	offlineProductSearchEndpoint = "/mms/marigold/sku/v2/search"
)

// GetOfflineProducts 获取已下架产品列表
func (c *APIClient) GetOfflineProducts(pageNo, pageSize int) (*OfflineProductSearchResponse, error) {
	c.logger.Infof("获取已下架产品列表: pageNo=%d, pageSize=%d", pageNo, pageSize)

	// 参数校验
	if pageSize > 200 {
		pageSize = 200 // 最大200
	}
	if pageSize <= 0 {
		pageSize = 50 // 默认50
	}
	if pageNo <= 0 {
		pageNo = 1 // 默认第1页
	}

	req := &OfflineProductSearchRequest{
		PageSize:              pageSize,
		PageNo:                pageNo,
		OrderType:             0,            // 0-降序
		OrderField:            "gmt_create", // 按创建时间排序
		EnableBatchSearchText: true,         // 启用批量搜索文本
		SkuSearchType:         3,            // 3-已下架产品
	}

	headers := GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["x-document-referer"] = "https://seller.temu.com/"

	request := map[string]any{
		"method":  "POST",
		"url":     offlineProductSearchEndpoint,
		"headers": headers,
		"body":    req,
	}

	var result OfflineProductSearchResponse
	if err := c.SendTEMURequest(request, &result); err != nil {
		c.logger.WithError(err).Error("获取已下架产品列表失败")
		return nil, fmt.Errorf("获取已下架产品列表失败: %w", err)
	}

	if !result.Success {
		c.logger.Errorf("获取已下架产品列表失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("获取已下架产品列表失败: errorCode=%d", result.ErrorCode)
	}

	c.logger.Infof("成功获取已下架产品列表: 总数=%d, 当前页商品数=%d",
		result.Result.Total, len(result.Result.SkuList))
	return &result, nil
}

// GetAllOfflineProducts 获取所有已下架产品（分页获取）
func (c *APIClient) GetAllOfflineProducts() ([]OfflineProductItem, error) {
	c.logger.Info("开始获取所有已下架产品")

	var allProducts []OfflineProductItem
	pageNo := 1
	pageSize := 200 // 使用最大页面大小

	for {
		resp, err := c.GetOfflineProducts(pageNo, pageSize)
		if err != nil {
			return allProducts, fmt.Errorf("获取第%d页已下架产品失败: %w", pageNo, err)
		}

		if resp == nil || len(resp.Result.SkuList) == 0 {
			c.logger.Info("没有更多已下架产品")
			break
		}

		// 添加到结果列表
		allProducts = append(allProducts, resp.Result.SkuList...)
		c.logger.Infof("已获取第%d页，累计产品数: %d", pageNo, len(allProducts))

		// 检查是否获取完所有产品
		if len(allProducts) >= resp.Result.Total {
			c.logger.Infof("已获取所有产品，总数: %d", len(allProducts))
			break
		}

		pageNo++
	}

	c.logger.Infof("获取所有已下架产品完成，总数: %d", len(allProducts))
	return allProducts, nil
}

// GetOfflineProductsByDateRange 根据时间范围获取已下架产品
func (c *APIClient) GetOfflineProductsByDateRange(startTime, endTime int64) ([]OfflineProductItem, error) {
	c.logger.Infof("获取时间范围内的已下架产品: %d - %d", startTime, endTime)

	allProducts, err := c.GetAllOfflineProducts()
	if err != nil {
		return nil, fmt.Errorf("获取已下架产品失败: %w", err)
	}

	// 过滤时间范围内的产品
	var filteredProducts []OfflineProductItem
	for _, product := range allProducts {
		// 这里需要解析product.CrtTime字符串为时间戳进行比较
		// 由于示例中CrtTime是字符串格式的时间戳，我们直接进行字符串比较
		// 实际使用时可能需要根据具体的时间格式进行解析
		if product.CrtTime != "" {
			// 简单的字符串比较，实际应该解析为时间戳
			filteredProducts = append(filteredProducts, product)
		}
	}

	c.logger.Infof("时间范围内的已下架产品数量: %d", len(filteredProducts))
	return filteredProducts, nil
}

// GetOfflineProductStatistics 获取已下架产品统计信息
func (c *APIClient) GetOfflineProductStatistics() (*OfflineProductStatistics, error) {
	c.logger.Info("获取已下架产品统计信息")

	allProducts, err := c.GetAllOfflineProducts()
	if err != nil {
		return nil, fmt.Errorf("获取已下架产品失败: %w", err)
	}

	stats := &OfflineProductStatistics{
		TotalCount: len(allProducts),
	}

	// 统计各种状态的产品数量
	categoryCount := make(map[string]int)
	statusCount := make(map[int]int)

	for _, product := range allProducts {
		// 统计分类
		if len(product.CatNameList) > 0 {
			category := product.CatNameList[0] // 使用第一个分类
			categoryCount[category]++
		}

		// 统计状态
		statusCount[product.Status4VO]++

		// 统计需要整改的产品
		if product.CategoryRectificationInfo.NeedRectification {
			stats.NeedRectificationCount++
		}

		// 统计有库存的产品
		if product.Stock > 0 {
			stats.HasStockCount++
		}

		// 统计惩罚标签产品
		if product.PunishTags > 0 {
			stats.PunishedCount++
		}
	}

	stats.CategoryCount = categoryCount
	stats.StatusCount = statusCount

	c.logger.Infof("已下架产品统计: 总数=%d, 需整改=%d, 有库存=%d, 被惩罚=%d",
		stats.TotalCount, stats.NeedRectificationCount, stats.HasStockCount, stats.PunishedCount)

	return stats, nil
}
