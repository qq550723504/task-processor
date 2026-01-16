// Package scheduler 提供SHEIN平台调度器相关服务
package scheduler

import (
	"encoding/json"
	"time"

	managementapi "task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/api/marketing"

	"github.com/sirupsen/logrus"
)

// buildActivityConfigs 构建活动配置列表（使用指定的折扣率和库存比例）
func (s *activityRegistrationServiceImpl) buildActivityConfigs(
	products []marketing.SkcInfo,
	dropRate int,
	stockRatio float64,
) []marketing.ActivityConfig {
	configList := make([]marketing.ActivityConfig, 0, len(products))

	for _, product := range products {
		// 跳过已配置的产品
		if product.IsConfigured {
			s.logger.Debugf("产品 [%s] 已配置，跳过", product.Skc)
			continue
		}

		// 计算活动库存
		actStock := s.calculateActivityStock(product.Stock, stockRatio)

		// 构建活动配置
		config := marketing.ActivityConfig{
			Skc:               product.Skc,
			ActStock:          actStock,
			DropRate:          dropRate,
			ReservedActStock:  0, // 不预留库存
			SitePriceInfoList: s.convertSitePriceInfoWithDiscount(product.SitePriceInfoList, float64(dropRate)/100),
		}

		configList = append(configList, config)
	}

	return configList
}

// buildActivityConfigsWithStrategy 根据策略构建活动配置列表（包含利润率检查）
func (s *activityRegistrationServiceImpl) buildActivityConfigsWithStrategy(
	products []marketing.SkcInfo,
	dropRate int,
	stockRatio float64,
	storeID int64,
) []marketing.ActivityConfig {
	// 先过滤掉利润率不足的产品
	filteredProducts := s.filterProductsByProfitMargin(products, float64(dropRate)/100, storeID)

	// 构建活动配置
	return s.buildActivityConfigs(filteredProducts, dropRate, stockRatio)
}

// buildActivityConfigsByProfit 根据最低利润率构建活动配置列表
func (s *activityRegistrationServiceImpl) buildActivityConfigsByProfit(
	products []marketing.SkcInfo,
	minProfitRate float64,
	stockRatio float64,
	storeID int64,
) []marketing.ActivityConfig {
	configList := make([]marketing.ActivityConfig, 0, len(products))

	// 确保利润率在有效范围内
	if minProfitRate <= 0 || minProfitRate >= 1 {
		minProfitRate = 0.15 // 默认15%利润率
	}

	// 从管理系统获取店铺所有产品数据（包含Attributes）
	productClient := s.managementClient.GetProductDataClient(storeID)
	shelfStatus := 2 // 2表示在售状态
	allProducts, err := productClient.ListByStore("SHEIN", 0, storeID, &shelfStatus)
	if err != nil {
		s.logger.WithError(err).Warn("获取店铺产品列表失败，无法使用利润率模式")
		return configList
	}

	// 构建 SKC -> Attributes 的映射
	skcAttributesMap := make(map[string]string)
	for _, prod := range allProducts {
		if prod.PlatformProductID != "" && prod.Attributes != "" {
			skcAttributesMap[prod.PlatformProductID] = prod.Attributes
		}
	}

	s.logger.Infof("从管理系统获取了 %d 个产品，构建了 %d 个SKC映射", len(allProducts), len(skcAttributesMap))

	successCount := 0
	skippedNoAttributes := 0
	skippedNoPrice := 0

	for _, product := range products {
		// 跳过已配置的产品
		if product.IsConfigured {
			s.logger.Debugf("产品 [%s] 已配置，跳过", product.SupplierNo)
			continue
		}

		// 从映射中获取Attributes
		attributes, exists := skcAttributesMap[product.Skc]
		if !exists || attributes == "" {
			s.logger.Debugf("产品 [%s] 没有Attributes数据，跳过", product.Skc)
			skippedNoAttributes++
			continue
		}

		// 从Attributes中提取Amazon价格作为成本价
		amazonPrice := s.extractAmazonPriceFromAttributes(attributes, product.Skc)
		if amazonPrice <= 0 {
			s.logger.Debugf("产品 [%s] 无法获取Amazon价格，跳过", product.Skc)
			skippedNoPrice++
			continue
		}

		// 计算活动库存
		actStock := s.calculateActivityStock(product.Stock, stockRatio)

		// 按最低利润率计算活动价格
		sitePriceInfoList := s.convertSitePriceInfoByProfitWithCost(product.SitePriceInfoList, minProfitRate, amazonPrice)

		// 如果没有有效的价格信息，跳过该产品
		if len(sitePriceInfoList) == 0 {
			s.logger.Debugf("产品 [%s] 无法按利润率定价，跳过", product.Skc)
			continue
		}

		// 计算等效的降价百分比（用于显示）
		dropRate := s.calculateEquivalentDropRate(product.SitePriceInfoList, sitePriceInfoList)

		// 构建活动配置
		config := marketing.ActivityConfig{
			Skc:               product.Skc,
			ActStock:          actStock,
			DropRate:          dropRate,
			ReservedActStock:  0, // 不预留库存
			SitePriceInfoList: sitePriceInfoList,
		}

		configList = append(configList, config)
		successCount++
	}

	s.logger.Infof("按利润率模式构建完成 - 成功: %d, 无Attributes: %d, 无Amazon价格: %d (最低利润率: %.2f%%)",
		successCount, skippedNoAttributes, skippedNoPrice, minProfitRate*100)

	return configList
}

// extractAmazonPriceFromAttributes 从Attributes中提取Amazon价格
func (s *activityRegistrationServiceImpl) extractAmazonPriceFromAttributes(attributesJSON string, skcCode string) float64 {
	if attributesJSON == "" {
		return 0
	}

	var skcList []EnrichedSkcInfo
	if err := json.Unmarshal([]byte(attributesJSON), &skcList); err != nil {
		s.logger.WithError(err).Debugf("解析产品Attributes失败: %s", skcCode)
		return 0
	}

	// 查找对应的SKC
	for _, skc := range skcList {
		if skc.SkcCode == skcCode {
			// 遍历SKU，查找Amazon监控数据
			for _, sku := range skc.SkuInfo {
				if sku.AmazonMonitorData != nil && sku.AmazonMonitorData.Price > 0 {
					s.logger.Debugf("产品 [%s] 找到Amazon价格: %.2f (ASIN: %s)",
						skcCode, sku.AmazonMonitorData.Price, sku.AmazonMonitorData.ASIN)
					return sku.AmazonMonitorData.Price
				}
			}
		}
	}

	s.logger.Debugf("产品 [%s] 未找到Amazon价格", skcCode)
	return 0
}

// convertSitePriceInfoByProfitWithCost 按最低利润率和实际成本价转换站点价格信息
func (s *activityRegistrationServiceImpl) convertSitePriceInfoByProfitWithCost(
	siteInfoList []marketing.SitePriceInfo,
	minProfitRate float64,
	costPrice float64,
) []marketing.ActivitySitePriceInfo {
	activitySiteInfoList := make([]marketing.ActivitySitePriceInfo, 0, len(siteInfoList))

	for _, siteInfo := range siteInfoList {
		// 按最低利润率和实际成本价计算活动价格
		activityPrice := calculatePriceByProfit(siteInfo.SalePrice, costPrice, minProfitRate)

		activitySiteInfo := marketing.ActivitySitePriceInfo{
			SiteCode:    siteInfo.SiteCode,
			SalePrice:   activityPrice,
			Currency:    siteInfo.Currency,
			IsAvailable: siteInfo.IsAvailable,
		}
		activitySiteInfoList = append(activitySiteInfoList, activitySiteInfo)
	}

	return activitySiteInfoList
}

// calculateEquivalentDropRate 计算等效的降价百分比
func (s *activityRegistrationServiceImpl) calculateEquivalentDropRate(
	originalPrices []marketing.SitePriceInfo,
	activityPrices []marketing.ActivitySitePriceInfo,
) int {
	if len(originalPrices) == 0 || len(activityPrices) == 0 {
		return 10 // 默认10%
	}

	// 使用第一个站点的价格计算等效折扣率
	originalPrice := originalPrices[0].SalePrice
	activityPrice := activityPrices[0].SalePrice

	if originalPrice <= 0 {
		return 10
	}

	// 计算折扣率
	discountRate := (originalPrice - activityPrice) / originalPrice
	dropRate := int(discountRate * 100)

	// 确保在合理范围内
	if dropRate < 0 {
		dropRate = 0
	} else if dropRate > 100 {
		dropRate = 100
	}

	return dropRate
}

// calculateActivityStock 计算活动库存
func (s *activityRegistrationServiceImpl) calculateActivityStock(totalStock int, stockRatio float64) int {
	// 确保库存比例在有效范围内
	if stockRatio <= 0 {
		stockRatio = 1.0 // 默认使用全部库存
	} else if stockRatio > 1.0 {
		stockRatio = 1.0
	}

	actStock := int(float64(totalStock) * stockRatio)

	// 至少保留1个库存用于活动
	if actStock < 1 && totalStock > 0 {
		actStock = 1
	}

	return actStock
}

// convertSitePriceInfo 转换站点价格信息（使用默认10%折扣）
func (s *activityRegistrationServiceImpl) convertSitePriceInfo(
	siteInfoList []marketing.SitePriceInfo,
) []marketing.ActivitySitePriceInfo {
	return s.convertSitePriceInfoWithDiscount(siteInfoList, 0.1)
}

// convertSitePriceInfoWithDiscount 转换站点价格信息（使用指定折扣率）
func (s *activityRegistrationServiceImpl) convertSitePriceInfoWithDiscount(
	siteInfoList []marketing.SitePriceInfo,
	discountRate float64,
) []marketing.ActivitySitePriceInfo {
	activitySiteInfoList := make([]marketing.ActivitySitePriceInfo, 0, len(siteInfoList))

	for _, siteInfo := range siteInfoList {
		// 计算活动价格：原价 * (1 - 折扣率)
		activityPrice := siteInfo.SalePrice * (1 - discountRate)

		activitySiteInfo := marketing.ActivitySitePriceInfo{
			SiteCode:    siteInfo.SiteCode,
			SalePrice:   activityPrice,
			Currency:    siteInfo.Currency,
			IsAvailable: siteInfo.IsAvailable,
		}
		activitySiteInfoList = append(activitySiteInfoList, activitySiteInfo)
	}

	return activitySiteInfoList
}

// buildTimeLimitedDiscountConfig 构建限时折扣配置
func (s *activityRegistrationServiceImpl) buildTimeLimitedDiscountConfig(
	storeInfo *managementapi.StoreRespDTO,
	strategy *managementapi.OperationStrategyDTO,
) TimeLimitedDiscountConfig {
	// 获取默认配置
	config := DefaultTimeLimitedDiscountConfig()

	// 生成活动名称（格式：#用户名#限时折扣#日期#序号）
	config.ActivityName = GenerateActivityName(storeInfo.Username, 1)

	// 设置活动时间（默认从现在开始，持续7天）
	now := time.Now()
	config.StartTime = now
	config.EndTime = now.AddDate(0, 0, 15)

	// 配置定价模式（优先级：限时折扣专属 > 通用配置 > 默认值）
	if strategy.TimeLimitedPriceMode != "" {
		config.PriceMode = strategy.TimeLimitedPriceMode
	}

	// 配置折扣率
	if strategy.TimeLimitedDiscountRate > 0 && strategy.TimeLimitedDiscountRate < 1 {
		config.DiscountRate = strategy.TimeLimitedDiscountRate
	}

	// 配置最低利润率（优先级：限时折扣专属 > 通用配置 > 默认值）
	if strategy.TimeLimitedMinProfitRate > 0 && strategy.TimeLimitedMinProfitRate < 1 {
		config.MinProfitRate = strategy.TimeLimitedMinProfitRate
	}

	// 配置单用户限购
	if strategy.TimeLimitedUserLimit {
		config.GoodsLimit = 1 // 启用限购
		if strategy.TimeLimitedUserLimitNum > 0 {
			config.GoodsLimitNum = strategy.TimeLimitedUserLimitNum
		}
	} else {
		config.GoodsLimit = 0 // 不限购
	}

	// 配置活动库存限量
	config.StockLimit = strategy.TimeLimitedStockLimit
	if strategy.TimeLimitedStockLimit && strategy.TimeLimitedStockLimitPercent > 0 && strategy.TimeLimitedStockLimitPercent <= 100 {
		config.StockPercent = strategy.TimeLimitedStockLimitPercent
	}

	s.logger.WithFields(logrus.Fields{
		"activity_name":   config.ActivityName,
		"start_time":      config.StartTime.Format("2006-01-02 15:04:05"),
		"end_time":        config.EndTime.Format("2006-01-02 15:04:05"),
		"price_mode":      config.PriceMode,
		"discount_rate":   config.DiscountRate,
		"min_profit_rate": config.MinProfitRate,
		"goods_limit":     config.GoodsLimit,
		"goods_limit_num": config.GoodsLimitNum,
		"stock_limit":     config.StockLimit,
		"stock_percent":   config.StockPercent,
		"default_stock":   config.DefaultStockNum,
	}).Debug("构建限时折扣配置完成")

	return config
}
