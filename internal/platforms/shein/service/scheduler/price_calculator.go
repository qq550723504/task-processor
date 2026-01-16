// Package scheduler 提供SHEIN平台价格计算相关服务
package scheduler

import (
	"encoding/json"

	"task-processor/internal/platforms/shein/api/marketing"
)

// calculateActivityPrice 根据定价模式计算活动价格
func calculateActivityPrice(
	config TimeLimitedDiscountConfig,
	originalPrice float64,
	costPrice float64,
) float64 {
	switch config.PriceMode {
	case "PROFIT":
		// 按最低利润率定价
		return calculatePriceByProfit(originalPrice, costPrice, config.MinProfitRate)
	case "DISCOUNT":
		// 按折扣率定价
		return calculatePriceByDiscount(originalPrice, config.DiscountRate)
	default:
		// 默认按折扣率定价
		return calculatePriceByProfit(originalPrice, costPrice, config.MinProfitRate)
	}
}

// calculatePriceByDiscount 按折扣率计算价格
func calculatePriceByDiscount(originalPrice float64, discountRate float64) float64 {
	return originalPrice * (1 - discountRate)
}

// calculatePriceByProfit 按最低利润率计算价格
func calculatePriceByProfit(originalPrice float64, costPrice float64, minProfitRate float64) float64 {
	// 计算最低售价 = 成本价 / (1 - 最低利润率)
	minPrice := costPrice / (1 - minProfitRate)

	// 如果原价低于最低售价
	if originalPrice < minPrice {
		return 0
	}

	// 计算活动价格：取原价和最低售价中的较大值
	activityPrice := minPrice
	if activityPrice > originalPrice {
		activityPrice = originalPrice
	}

	return activityPrice
}

// calculateProfitRate 计算利润率
func calculateProfitRate(salePrice float64, costPrice float64) float64 {
	if salePrice <= 0 {
		return 0
	}
	return (salePrice - costPrice) / salePrice
}

// buildCalculateRequestWithPriceMode 根据定价模式构建价格计算请求
func (s *activityRegistrationServiceImpl) buildCalculateRequestWithPriceMode(
	config TimeLimitedDiscountConfig,
	goods []marketing.PromotionGoodsData,
	storeID int64,
) *marketing.CalculateSupplyPriceRequest {
	skcInfoList := make([]marketing.SkcPriceInfo, 0, len(goods))

	// 如果使用利润率模式,需要获取产品的Attributes来提取Amazon价格
	var skcDataMap map[string]*EnrichedSkcInfo
	if config.PriceMode == "PROFIT" {
		productClient := s.managementClient.GetProductDataClient(storeID)
		shelfStatus := 2 // 2表示在售状态
		allProducts, err := productClient.ListByStore("SHEIN", 0, storeID, &shelfStatus)
		if err != nil {
			s.logger.WithError(err).Warn("获取店铺产品列表失败，无法使用利润率模式")
		} else {
			// 构建 SKC -> EnrichedSkcInfo 的映射
			skcDataMap = make(map[string]*EnrichedSkcInfo)
			for _, prod := range allProducts {
				if prod.Attributes == "" {
					continue
				}

				// 解析Attributes,提取每个SKC的数据
				var skcList []EnrichedSkcInfo
				if err := json.Unmarshal([]byte(prod.Attributes), &skcList); err != nil {
					s.logger.WithError(err).Debugf("解析产品Attributes失败: %s", prod.ProductID)
					continue
				}

				// 为每个SKC建立映射
				for i := range skcList {
					if skcList[i].SkcName != "" {
						skcDataMap[skcList[i].SkcName] = &skcList[i]
					}
				}
			}
			s.logger.Warningf("构建了 %d 个SKC的数据映射", len(skcDataMap))
		}
	}

	for _, g := range goods {
		// 根据定价模式计算价格
		var discountValue float64
		if config.PriceMode == "PROFIT" && skcDataMap != nil {
			// 从映射中获取SKC数据
			var costPrice float64
			if skcData, exists := skcDataMap[g.Skc]; exists {
				// 从SKC的SKU列表中提取Amazon价格
				costPrice = s.extractAmazonPriceFromSkcData(skcData)
			}

			if costPrice > 0 {
				// 按最低利润率和实际成本价计算
				discountValue = calculatePriceByProfit(g.USSupplyPrice, costPrice, config.MinProfitRate)
				if discountValue <= 0 {
					// 利润率不足 - 计算实际需要的最低售价
					minPrice := costPrice / (1 - config.MinProfitRate)
					s.logger.Warningf("商品 %s 利润率不足 (原价: %.2f, 成本: %.2f, 最低售价: %.2f, 要求利润率: %.2f%%)，跳过",
						g.Skc, g.USSupplyPrice, costPrice, minPrice, config.MinProfitRate*100)
				}
			} else {
				// 无法获取成本价
				s.logger.Warningf("商品 %s 无法获取Amazon成本价 (原价: %.2f)，跳过", g.Skc, g.USSupplyPrice)
			}
		} else {
			// 按折扣率计算
			discountValue = calculatePriceByDiscount(g.USSupplyPrice, config.DiscountRate)
		}

		// 如果活动价格为0或负数,跳过该商品(利润不足或无法定价)
		if discountValue <= 0 {
			s.logger.Warningf("商品 %s 活动价格为 %.2f (原价: %.2f)，跳过", g.Skc, discountValue, g.USSupplyPrice)
			continue
		}

		// 构建SKU价格列表
		skuInfoList := make([]marketing.SkuPriceInfo, 0, len(g.SkuInfoList))
		for _, sku := range g.SkuInfoList {
			skuInfoList = append(skuInfoList, marketing.SkuPriceInfo{
				SkuCode:       sku.Sku,
				ProductPrice:  g.USSupplyPrice,
				DiscountValue: discountValue,
			})
		}

		skcInfoList = append(skcInfoList, marketing.SkcPriceInfo{
			SkcName:     g.Skc,
			SkuInfoList: skuInfoList,
		})
	}

	return &marketing.CalculateSupplyPriceRequest{
		Currency:      config.Currency,
		RefToolID:     config.RefToolID,
		SceneID:       config.SceneID,
		SkcInfoList:   skcInfoList,
		TimeZone:      config.TimeZone,
		ZoneStartTime: config.StartTime.Format("2006-01-02 15:04:05"),
		ZoneEndTime:   config.EndTime.Format("2006-01-02 15:04:05"),
	}
}

// extractAmazonPriceFromSkcData 从EnrichedSkcInfo中提取Amazon价格
func (s *activityRegistrationServiceImpl) extractAmazonPriceFromSkcData(skcData *EnrichedSkcInfo) float64 {
	if skcData == nil {
		return 0
	}

	// 遍历SKU，查找Amazon监控数据
	for _, sku := range skcData.SkuInfo {
		if sku.AmazonMonitorData != nil && sku.AmazonMonitorData.Price > 0 {
			s.logger.Debugf("产品 [%s] 找到Amazon价格: %.2f (ASIN: %s)",
				skcData.SkcCode, sku.AmazonMonitorData.Price, sku.AmazonMonitorData.ASIN)
			return sku.AmazonMonitorData.Price
		}
	}

	return 0
}
