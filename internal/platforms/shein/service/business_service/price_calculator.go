// Package scheduler 提供SHEIN平台价格计算相关服务
package scheduler

import (
	"task-processor/internal/platforms/shein/api/marketing"

	"github.com/sirupsen/logrus"
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
		return calculatePriceByProfit(originalPrice, costPrice, config.MinProfitRate, config.FixedPriceAdjustment)
	case "DISCOUNT":
		// 按折扣率定价
		return calculatePriceByDiscount(originalPrice, config.DiscountRate)
	default:
		// 默认按折扣率定价
		return calculatePriceByProfit(originalPrice, costPrice, config.MinProfitRate, config.FixedPriceAdjustment)
	}
}

// calculatePriceByDiscount 按折扣率计算价格
func calculatePriceByDiscount(originalPrice float64, discountRate float64) float64 {
	return originalPrice * (1 - discountRate)
}

// calculatePriceByProfit 按最低利润率计算价格
func calculatePriceByProfit(originalPrice float64, costPrice float64, minProfitRate float64, fixedAdjustment float64) float64 {
	// 计算最低售价 = 成本价 / (1 - 最低利润率) + 固定调整值
	minPrice := costPrice/(1-minProfitRate) + fixedAdjustment

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

	// 统计价格计算阶段的过滤情况
	var (
		totalInputGoods    = len(goods)
		skippedByProfit    = 0
		skippedByCostPrice = 0
		skippedByZeroPrice = 0
	)

	s.logger.Infof("开始构建价格计算请求，输入商品数量: %d，定价模式: %s", totalInputGoods, config.PriceMode)

	// 如果使用利润率模式,需要获取产品的Attributes来提取Amazon价格
	var skcDataMap map[string]*EnrichedSkcInfo
	var helper *ProductDataHelper
	if config.PriceMode == "PROFIT" {
		helper = NewProductDataHelper(s.managementClient, s.logger.Logger)
		var err error
		skcDataMap, err = helper.BuildSkcDataMap(storeID)
		if err != nil {
			s.logger.WithError(err).Warn("构建SKC数据映射失败，无法使用利润率模式")
		}
	}

	for _, g := range goods {
		// 根据定价模式计算价格
		var discountValue float64
		if config.PriceMode == "PROFIT" && skcDataMap != nil {
			// 从映射中获取SKC数据
			var costPrice float64
			if skcData, exists := skcDataMap[g.Skc]; exists {
				// 使用助手函数提取Amazon价格
				costPrice = helper.ExtractAmazonPriceFromSkcData(skcData)
			}

			if costPrice > 0 {
				// 按最低利润率和实际成本价计算
				discountValue = calculatePriceByProfit(g.USSupplyPrice, costPrice, config.MinProfitRate, config.FixedPriceAdjustment)
				if discountValue <= 0 {
					// 利润率不足 - 计算实际需要的最低售价并添加固定调整值
					minPrice := costPrice/(1-config.MinProfitRate) + config.FixedPriceAdjustment
					s.logger.Warnf("商品 %s 利润率不足 (原价: %.2f, 成本: %.2f, 最低售价: %.2f, 固定调整: %.2f, 要求利润率: %.2f%%)，跳过",
						g.Skc, g.USSupplyPrice, costPrice, minPrice, config.FixedPriceAdjustment, config.MinProfitRate*100)
					skippedByProfit++
				}
			} else {
				// 无法获取成本价
				s.logger.Warnf("商品 %s 无法获取Amazon成本价 (原价: %.2f)，跳过", g.Skc, g.USSupplyPrice)
				skippedByCostPrice++
			}
		} else {
			// 按折扣率计算
			discountValue = calculatePriceByDiscount(g.USSupplyPrice, config.DiscountRate)
		}

		// 如果活动价格为0或负数,跳过该商品(利润不足或无法定价)
		if discountValue <= 0 {
			s.logger.Warnf("商品 %s 活动价格为 %.2f (原价: %.2f)，跳过", g.Skc, discountValue, g.USSupplyPrice)
			skippedByZeroPrice++
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

	// 输出价格计算阶段的统计信息
	s.logger.WithFields(logrus.Fields{
		"total_input_goods":     totalInputGoods,
		"skipped_by_profit":     skippedByProfit,
		"skipped_by_cost_price": skippedByCostPrice,
		"skipped_by_zero_price": skippedByZeroPrice,
		"final_calc_goods":      len(skcInfoList),
	}).Info("价格计算阶段统计")

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
