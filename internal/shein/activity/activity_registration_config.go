// package activity 提供SHEIN平台调度器相关服务
package activity

import (
	"strconv"
	"time"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/shein/api/marketing"
	"task-processor/internal/shein/operation"

	"github.com/sirupsen/logrus"
)

// buildActivityConfigs 构建活动配置列表（使用指定的折扣率和库存比例）
func (s *activityRegistrationServiceImpl) buildActivityConfigs(
	products []marketing.SkcInfo,
	dropRate int,
	stockRatio float64,
	storeID int64,
) []marketing.ActivityConfig {
	configList := make([]marketing.ActivityConfig, 0, len(products))

	// 使用公共的ProductDataHelper构建SKC属性映射
	helper := NewProductDataHelper(s.managementClient, s.logger.Logger)
	skcAttributesMap, err := helper.BuildSkcAttributesMap(storeID)
	if err != nil {
		s.logger.WithError(err).Warn("构建SKC属性映射失败，使用原有逻辑")
	}

	successCount := 0
	skippedConfigured := 0
	skippedNoAttributes := 0
	skippedNoPriceData := 0

	for _, product := range products {
		// 跳过已配置的产品
		if product.IsConfigured {
			s.logger.Debugf("产品 [%s] 已配置，跳过", product.Skc)
			skippedConfigured++
			continue
		}

		// 从映射中获取Attributes
		attributes, exists := skcAttributesMap[product.Skc]
		if !exists || attributes == "" {
			s.logger.Warningf("产品 [%s] 没有Attributes数据，跳过", product.Skc)
			skippedNoAttributes++
			continue
		}

		// 使用公共方法从Attributes中提取SKC的完整信息
		skcInfo := helper.ExtractSkcInfoFromAttributes(attributes, product.Skc)
		if skcInfo == nil {
			s.logger.Warningf("产品 [%s] 无法从Attributes中获取SKC信息，跳过", product.Skc)
			skippedNoAttributes++
			continue
		}

		// 验证该SKC下的所有SKU是否都有必要的价格数据
		hasValidPriceData := false
		for _, sku := range skcInfo.SkuInfo {
			// 检查Amazon价格数据
			hasAmazonPrice := sku.AmazonMonitorData != nil && sku.AmazonMonitorData.Price > 0

			// 检查映射价格信息
			var hasMappingPrice bool
			if sku.MappingInfo != nil && sku.MappingInfo.CostPrice != nil && *sku.MappingInfo.CostPrice > 0 {
				if parsedPrice, parseErr := strconv.ParseFloat(sku.CostPriceInfo.CostPrice, 64); parseErr == nil && parsedPrice > 0 {
					hasMappingPrice = true
				}
			}

			// 只要有一个SKU有完整的价格数据，就认为这个SKC可用
			if hasAmazonPrice && hasMappingPrice {
				hasValidPriceData = true
				break
			}
		}

		if !hasValidPriceData {
			s.logger.Warningf("产品 [%s] 没有有效的价格数据，跳过", product.Skc)
			skippedNoPriceData++
			continue
		}

		// 计算活动库存
		actStock := s.calculateActivityStock(product.Stock, stockRatio)

		// 确保活动库存和预留库存都是正整数
		if actStock <= 0 {
			s.logger.Warnf("产品 [%s] 活动库存为0，跳过", product.Skc)
			skippedNoPriceData++
			continue
		}

		reservedStock := product.Stock
		if reservedStock <= 0 {
			s.logger.Warnf("产品 [%s] 预留库存为0，跳过", product.Skc)
			skippedNoPriceData++
			continue
		}

		sitePriceInfoList := []marketing.ActivitySitePriceInfo{}
		// 构建活动配置
		config := marketing.ActivityConfig{
			Skc:               product.Skc,
			ActStock:          actStock,
			DropRate:          dropRate,
			ReservedActStock:  reservedStock,
			SitePriceInfoList: sitePriceInfoList,
		}

		configList = append(configList, config)
		successCount++
	}

	s.logger.Infof("按固定折扣率模式构建完成 - 成功: %d, 已配置: %d, 无Attributes: %d, 无价格数据: %d (折扣率: %d%%)",
		successCount, skippedConfigured, skippedNoAttributes, skippedNoPriceData, dropRate)

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
	return s.buildActivityConfigs(filteredProducts, dropRate, stockRatio, storeID)
}

// buildActivityConfigsByProfit 根据最低利润率构建活动配置列表
func (s *activityRegistrationServiceImpl) buildActivityConfigsByProfit(
	products []marketing.SkcInfo,
	minProfitRate float64,
	stockRatio float64,
	storeID int64,
	fixedPriceAdjustment float64, // 添加固定价格调整值参数
) []marketing.ActivityConfig {
	configList := make([]marketing.ActivityConfig, 0, len(products))

	// 确保利润率在有效范围内
	if minProfitRate <= 0 || minProfitRate >= 1 {
		minProfitRate = 0.15 // 默认15%利润率
	}

	// 【调试代码】记录已添加的商品数量
	addedCount := 0
	maxProductsPerActivity := 500 // SHEIN平台限制:一次活动最多500个商品

	// 使用公共的ProductDataHelper构建SKC属性映射
	helper := NewProductDataHelper(s.managementClient, s.logger.Logger)
	skcAttributesMap, err := helper.BuildSkcAttributesMap(storeID)
	if err != nil {
		s.logger.WithError(err).Warn("构建SKC属性映射失败，无法使用利润率模式")
		return configList
	}

	successCount := 0
	skippedNoAttributes := 0
	skippedInsufficientProfit := 0

	for _, product := range products {
		// 检查是否已达到商品数量上限
		if addedCount >= maxProductsPerActivity {
			s.logger.Warnf("已达到单次活动商品数量上限(%d),停止添加商品", maxProductsPerActivity)
			break
		}

		// 跳过已配置的产品
		if product.IsConfigured {
			s.logger.Warningf("产品 [%s] 已配置，跳过", product.SupplierNo)
			continue
		}

		// 从映射中获取Attributes
		attributes, exists := skcAttributesMap[product.Skc]
		if !exists || attributes == "" {
			s.logger.Warningf("产品 [%s] 没有Attributes数据，跳过", product.Skc)
			skippedNoAttributes++
			continue
		}

		// 使用公共方法从Attributes中提取SKC的完整信息
		skcInfo := helper.ExtractSkcInfoFromAttributes(attributes, product.Skc)
		if skcInfo == nil {
			s.logger.Warningf("产品 [%s] 无法从Attributes中获取SKC信息，跳过", product.Skc)
			skippedNoAttributes++
			continue
		}

		// 检查该SKC下的所有SKU是否都满足利润率要求
		validSkus := make([]operation.EnrichedSkuInfo, 0, len(skcInfo.SkuInfo))
		totalOriginalPrice := 0.0
		totalActivityPrice := 0.0
		skuCount := 0
		allSkusValid := true

		for _, sku := range skcInfo.SkuInfo {
			// 获取Amazon价格作为成本价
			var costPrice float64
			if sku.AmazonMonitorData != nil && sku.AmazonMonitorData.Price > 0 {
				costPrice = sku.AmazonMonitorData.Price
			} else {
				s.logger.Warningf("SKU [%s] 无Amazon价格数据，跳过", sku.SkuCode)
				continue
			}

			// 获取映射信息中的原价
			var originalPrice float64
			if sku.MappingInfo != nil && sku.MappingInfo.CostPrice != nil && *sku.MappingInfo.CostPrice > 0 {
				// 半托从CostPriceInfo中解析价格字符串，//todo:自营全托
				if parsedPrice, err := strconv.ParseFloat(sku.CostPriceInfo.CostPrice, 64); err == nil && parsedPrice > 0 {
					originalPrice = parsedPrice
				} else {
					s.logger.Warningf("SKU [%s] 价格解析失败: %s", sku.SkuCode, sku.CostPriceInfo.CostPrice)
					continue
				}
			} else {
				s.logger.Warningf("SKU [%s] 无映射价格信息，跳过", sku.SkuCode)
				continue
			}

			// 按最低利润率计算活动价格（使用固定价格调整值）
			activityPrice := calculatePriceByProfit(originalPrice, costPrice, minProfitRate, fixedPriceAdjustment)
			if activityPrice <= 0 {
				s.logger.Warningf("SKU [%s] 利润率不足 (原价: %.2f, 成本: %.2f, 要求利润率: %.2f%%, 固定调整: %.2f)，整个SKC跳过",
					sku.SkuCode, originalPrice, costPrice, minProfitRate*100, fixedPriceAdjustment)
				// 如果有任何一个SKU不满足利润率要求，整个SKC都跳过
				allSkusValid = false
				break
			}

			validSkus = append(validSkus, sku)
			totalOriginalPrice += originalPrice
			totalActivityPrice += activityPrice
			skuCount++
		}

		// 如果不是所有SKU都满足要求，或者没有有效的SKU，跳过该产品
		if !allSkusValid || len(validSkus) == 0 {
			s.logger.Warningf("产品 [%s] 没有满足利润率要求的SKU，跳过", product.Skc)
			skippedInsufficientProfit++
			continue
		}

		// 计算平均折扣率
		avgOriginalPrice := totalOriginalPrice / float64(skuCount)
		avgActivityPrice := totalActivityPrice / float64(skuCount)
		discountRate := (avgOriginalPrice - avgActivityPrice) / avgOriginalPrice
		dropRate := int(discountRate * 100)

		// 确保折扣率在合理范围内（SHEIN API要求1-99）
		dropRate = ValidateDropRate(dropRate, discountRate, s.logger)

		// 计算活动库存
		actStock := s.calculateActivityStock(product.Stock, stockRatio)

		// 确保活动库存和预留库存都是正整数
		if actStock <= 0 {
			s.logger.Warnf("产品 [%s] 活动库存为0，跳过", product.Skc)
			skippedInsufficientProfit++
			continue
		}

		reservedStock := product.Stock
		if reservedStock <= 0 {
			s.logger.Warnf("产品 [%s] 预留库存为0，跳过", product.Skc)
			skippedInsufficientProfit++
			continue
		}

		// 由于Attributes中没有SitePriceInfoList，且提交时可以为空，我们构造一个空的站点价格信息列表
		sitePriceInfoList := []marketing.ActivitySitePriceInfo{}

		// 构建活动配置
		config := marketing.ActivityConfig{
			Skc:               product.Skc,
			ActStock:          actStock,
			DropRate:          dropRate,
			ReservedActStock:  reservedStock,
			SitePriceInfoList: sitePriceInfoList,
		}

		configList = append(configList, config)
		successCount++
		addedCount++ // 增加已添加商品计数
	}

	s.logger.Infof("按利润率模式构建完成 - 成功: %d, 无Attributes: %d, 利润率不足: %d (最低利润率: %.2f%%, 最大限制: %d)",
		successCount, skippedNoAttributes, skippedInsufficientProfit, minProfitRate*100, maxProductsPerActivity)

	return configList
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

	// 确保在合理范围内（SHEIN API要求1-99）
	dropRate = ValidateDropRate(dropRate, discountRate, s.logger)

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

	// 配置固定价格调整值
	config.FixedPriceAdjustment = strategy.FixedPriceAdjustment

	s.logger.WithFields(logrus.Fields{
		"activity_name":          config.ActivityName,
		"start_time":             config.StartTime.Format("2006-01-02 15:04:05"),
		"end_time":               config.EndTime.Format("2006-01-02 15:04:05"),
		"price_mode":             config.PriceMode,
		"discount_rate":          config.DiscountRate,
		"min_profit_rate":        config.MinProfitRate,
		"goods_limit":            config.GoodsLimit,
		"goods_limit_num":        config.GoodsLimitNum,
		"stock_limit":            config.StockLimit,
		"stock_percent":          config.StockPercent,
		"default_stock":          config.DefaultStockNum,
		"fixed_price_adjustment": config.FixedPriceAdjustment,
	}).Debug("构建限时折扣配置完成")

	return config
}
