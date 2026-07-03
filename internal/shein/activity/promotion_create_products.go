package activity

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/listingruntime"
	"task-processor/internal/shein/api/marketing"

	"github.com/sirupsen/logrus"
)

func (s *activityRegistrationServiceImpl) createPromotionActivityFromProducts(
	ctx context.Context,
	strategy *listingruntime.OperationStrategy,
	activityKey string,
	products []marketing.SkcInfo,
) (*PromotionRegistrationResult, error) {
	if strategy == nil {
		return nil, fmt.Errorf("operation strategy is required")
	}
	storeInfo, err := s.getStoreInfo(ctx, strategy.StoreID)
	if err != nil {
		return nil, fmt.Errorf("获取店铺信息失败: %w", err)
	}

	config := s.buildPromotionCreateConfig(storeInfo, strategy, activityKey, products)
	allGoods, err := s.queryAllPromotionGoods(config)
	if err != nil {
		return nil, fmt.Errorf("查询商品失败: %w", err)
	}
	return s.createPromotionActivityFromPreparedGoods(ctx, strategy, config, products, allGoods)
}

func (s *activityRegistrationServiceImpl) NewPromotionRegistrationSession(
	ctx context.Context,
	strategy *listingruntime.OperationStrategy,
	activityKey string,
) (PromotionRegistrationSession, error) {
	if strategy == nil {
		return nil, fmt.Errorf("operation strategy is required")
	}
	storeInfo, err := s.getStoreInfo(ctx, strategy.StoreID)
	if err != nil {
		return nil, fmt.Errorf("获取店铺信息失败: %w", err)
	}
	config := s.buildPromotionCreateConfig(storeInfo, strategy, activityKey, nil)
	allGoods, err := s.queryAllPromotionGoods(config)
	if err != nil {
		return nil, fmt.Errorf("查询商品失败: %w", err)
	}
	return &promotionRegistrationSession{
		service:   s,
		strategy:  clonePromotionOperationStrategy(strategy),
		storeInfo: storeInfo,
		allGoods:  append([]marketing.PromotionGoodsData(nil), allGoods...),
	}, nil
}

type promotionRegistrationSession struct {
	service   *activityRegistrationServiceImpl
	strategy  *listingruntime.OperationStrategy
	storeInfo *listingruntime.StoreInfo
	allGoods  []marketing.PromotionGoodsData
}

func (s *promotionRegistrationSession) RegisterPromotionProducts(
	ctx context.Context,
	activityKey string,
	products []marketing.SkcInfo,
) (*PromotionRegistrationResult, error) {
	if s == nil || s.service == nil {
		return nil, fmt.Errorf("promotion registration session is required")
	}
	if len(products) == 0 {
		return &PromotionRegistrationResult{}, nil
	}
	config := s.service.buildPromotionCreateConfig(s.storeInfo, s.strategy, activityKey, products)
	return s.service.createPromotionActivityFromPreparedGoods(ctx, s.strategy, config, products, s.allGoods)
}

func (s *activityRegistrationServiceImpl) buildPromotionCreateConfig(
	storeInfo *listingruntime.StoreInfo,
	strategy *listingruntime.OperationStrategy,
	activityKey string,
	products []marketing.SkcInfo,
) TimeLimitedDiscountConfig {
	config := s.buildTimeLimitedDiscountConfig(storeInfo, strategy, activityKey)
	applyPromotionCreateConfig(&config, strategy)
	config.FilterSkcList = selectedPromotionSKCs(products)
	return config
}

func (s *activityRegistrationServiceImpl) createPromotionActivityFromPreparedGoods(
	ctx context.Context,
	strategy *listingruntime.OperationStrategy,
	config TimeLimitedDiscountConfig,
	products []marketing.SkcInfo,
	allGoods []marketing.PromotionGoodsData,
) (*PromotionRegistrationResult, error) {
	if validateErr := config.Validate(); validateErr != nil {
		return nil, fmt.Errorf("配置验证失败: %w", validateErr)
	}
	goods := filterPromotionGoodsBySKC(allGoods, config.FilterSkcList)
	if len(goods) == 0 {
		return &PromotionRegistrationResult{}, ErrNoAvailableProducts
	}
	goods = enrichPromotionGoodsFromProductSnapshots(goods, products, config.Currency)

	calcReq := s.buildCalculateRequestForPromotionProducts(config, goods, products)
	if len(calcReq.SkcInfoList) == 0 {
		return &PromotionRegistrationResult{}, ErrNoAvailableProducts
	}
	calcResp, err := s.calculateSupplyPrice(calcReq)
	if err != nil {
		return nil, fmt.Errorf("计算价格失败: %w", err)
	}
	if riskErr := s.validatePriceRisk(calcResp, config); riskErr != nil {
		return nil, fmt.Errorf("价格风险检查失败: %w", riskErr)
	}

	createReq, filterReasons, filterReasonBySKC := s.buildCreateActivityRequest(config, goods, calcReq, calcResp)
	if len(createReq.AddCostAndStockInfoList) == 0 {
		return &PromotionRegistrationResult{ActivityRequest: createReq, FilterReasons: filterReasonBySKC}, noAvailablePromotionProductsError(filterReasons)
	}
	createResp, err := s.createTimeLimitedDiscount(createReq)
	result := &PromotionRegistrationResult{
		ActivityRequest:  createReq,
		ActivityResponse: createResp,
		FilterReasons:    filterReasonBySKC,
	}
	if err != nil {
		return result, fmt.Errorf("创建活动失败: %w", err)
	}
	if err := s.checkCreateResult(createResp); err != nil {
		return result, fmt.Errorf("活动创建结果异常: %w", err)
	}
	if createResp != nil && createResp.Info != nil {
		s.logger.WithFields(logrus.Fields{
			"activity_name": config.ActivityName,
			"activity_id":   createResp.Info.ActivityID,
			"skc_count":     len(createReq.AddCostAndStockInfoList),
		}).Info("成功创建 SHEIN 促销活动")
	}
	return result, nil
}

func clonePromotionOperationStrategy(source *listingruntime.OperationStrategy) *listingruntime.OperationStrategy {
	if source == nil {
		return nil
	}
	copied := *source
	return &copied
}

func applyPromotionCreateConfig(config *TimeLimitedDiscountConfig, strategy *listingruntime.OperationStrategy) {
	if config == nil || strategy == nil {
		return
	}
	priceMode := strings.ToUpper(strings.TrimSpace(strategy.ActivityPriceMode))
	if priceMode != "" {
		config.PriceMode = priceMode
	}
	if strategy.ActivityDiscountRate > 0 && strategy.ActivityDiscountRate < 1 {
		config.DiscountRate = strategy.ActivityDiscountRate
	}
	if strategy.ActivityMinProfitRate >= 0 && strategy.ActivityMinProfitRate < 1 {
		config.MinProfitRate = strategy.ActivityMinProfitRate
	}
	if strategy.ActivityStockRatio > 0 && strategy.ActivityStockRatio <= 1 {
		config.StockLimit = true
		config.StockPercent = int(strategy.ActivityStockRatio * 100)
		if config.StockPercent < 1 {
			config.StockPercent = 1
		}
	}
}

func selectedPromotionSKCs(products []marketing.SkcInfo) []string {
	selected := make([]string, 0, len(products))
	seen := make(map[string]struct{}, len(products))
	for _, product := range products {
		skc := strings.TrimSpace(product.Skc)
		if skc == "" {
			continue
		}
		if _, ok := seen[skc]; ok {
			continue
		}
		seen[skc] = struct{}{}
		selected = append(selected, skc)
	}
	return selected
}

func filterPromotionGoodsBySKC(goods []marketing.PromotionGoodsData, selected []string) []marketing.PromotionGoodsData {
	if len(selected) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(selected))
	for _, skc := range selected {
		allowed[skc] = struct{}{}
	}
	filtered := make([]marketing.PromotionGoodsData, 0, len(goods))
	for _, item := range goods {
		if _, ok := allowed[item.Skc]; ok {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func enrichPromotionGoodsFromProductSnapshots(
	goods []marketing.PromotionGoodsData,
	products []marketing.SkcInfo,
	currency string,
) []marketing.PromotionGoodsData {
	productBySKC := make(map[string]marketing.SkcInfo, len(products))
	for _, product := range products {
		if strings.TrimSpace(product.Skc) != "" {
			productBySKC[product.Skc] = product
		}
	}

	enriched := make([]marketing.PromotionGoodsData, 0, len(goods))
	for _, item := range goods {
		product, ok := productBySKC[item.Skc]
		if !ok {
			enriched = append(enriched, item)
			continue
		}
		fallbackPrice := promotionProductSalePrice(product, currency)
		if fallbackPrice <= 0 {
			fallbackPrice = promotionProductSKUFallbackSalePrice(product, currency)
		}
		if item.USSupplyPrice <= 0 && fallbackPrice > 0 {
			item.USSupplyPrice = fallbackPrice
		}
		if item.MaxUSSupplyPrice <= 0 && item.USSupplyPrice > 0 {
			item.MaxUSSupplyPrice = item.USSupplyPrice
		}
		if item.InventoryNum <= 0 && product.Stock > 0 {
			item.InventoryNum = product.Stock
		}
		item.SkuInfoList = enrichPromotionSKUPricesFromProductSnapshot(item.SkuInfoList, product, configCurrencyOrDefault(currency))
		enriched = append(enriched, item)
	}
	return enriched
}

func configCurrencyOrDefault(currency string) string {
	currency = strings.TrimSpace(strings.ToUpper(currency))
	if currency == "" {
		return "USD"
	}
	return currency
}

func enrichPromotionSKUPricesFromProductSnapshot(
	skus []marketing.PromotionSkuInfo,
	product marketing.SkcInfo,
	preferredCurrency string,
) []marketing.PromotionSkuInfo {
	if len(skus) == 0 || len(product.SkuPriceInfoList) == 0 {
		return skus
	}
	priceBySKU := make(map[string]marketing.SitePriceInfo, len(product.SkuPriceInfoList))
	for _, skuPrice := range product.SkuPriceInfoList {
		skuCode := strings.TrimSpace(skuPrice.SkuCode)
		if skuCode == "" {
			continue
		}
		if sitePrice, ok := promotionPreferredSitePrice(skuPrice.SitePriceInfoList, preferredCurrency); ok {
			priceBySKU[skuCode] = sitePrice
		}
	}
	if len(priceBySKU) == 0 {
		return skus
	}
	out := append([]marketing.PromotionSkuInfo(nil), skus...)
	for idx := range out {
		sitePrice, ok := priceBySKU[out[idx].Sku]
		if !ok || sitePrice.SalePrice <= 0 {
			continue
		}
		if out[idx].USSupplyPrice == nil || *out[idx].USSupplyPrice <= 0 {
			out[idx].USSupplyPrice = promotionFloat64Ptr(sitePrice.SalePrice)
		}
		if out[idx].SupplyPrice == nil || *out[idx].SupplyPrice <= 0 {
			out[idx].SupplyPrice = promotionFloat64Ptr(sitePrice.SalePrice)
		}
		if out[idx].MaxUSSupplyPrice == nil || *out[idx].MaxUSSupplyPrice <= 0 {
			out[idx].MaxUSSupplyPrice = promotionFloat64Ptr(sitePrice.SalePrice)
		}
		if out[idx].MaxSupplyPrice == nil || *out[idx].MaxSupplyPrice <= 0 {
			out[idx].MaxSupplyPrice = promotionFloat64Ptr(sitePrice.SalePrice)
		}
	}
	return out
}

func promotionPreferredSitePrice(items []marketing.SitePriceInfo, preferredCurrency string) (marketing.SitePriceInfo, bool) {
	for _, item := range items {
		if item.SalePrice > 0 && item.IsAvailable && strings.EqualFold(item.Currency, preferredCurrency) {
			return item, true
		}
	}
	for _, item := range items {
		if item.SalePrice > 0 && item.IsAvailable {
			return item, true
		}
	}
	for _, item := range items {
		if item.SalePrice > 0 {
			return item, true
		}
	}
	return marketing.SitePriceInfo{}, false
}

func promotionFloat64Ptr(value float64) *float64 {
	return &value
}

func promotionProductSalePrice(product marketing.SkcInfo, preferredCurrency string) float64 {
	preferredCurrency = strings.TrimSpace(strings.ToUpper(preferredCurrency))
	for _, site := range product.SitePriceInfoList {
		if site.SalePrice > 0 && site.IsAvailable && strings.EqualFold(site.Currency, preferredCurrency) {
			return site.SalePrice
		}
	}
	for _, site := range product.SitePriceInfoList {
		if site.SalePrice > 0 && site.IsAvailable {
			return site.SalePrice
		}
	}
	for _, site := range product.SitePriceInfoList {
		if site.SalePrice > 0 {
			return site.SalePrice
		}
	}
	return 0
}

func promotionProductSKUFallbackSalePrice(product marketing.SkcInfo, preferredCurrency string) float64 {
	preferredCurrency = strings.TrimSpace(strings.ToUpper(preferredCurrency))
	for _, skuPrice := range product.SkuPriceInfoList {
		if sitePrice, ok := promotionPreferredSitePrice(skuPrice.SitePriceInfoList, preferredCurrency); ok {
			return sitePrice.SalePrice
		}
	}
	return 0
}

func (s *activityRegistrationServiceImpl) buildCalculateRequestForPromotionProducts(
	config TimeLimitedDiscountConfig,
	goods []marketing.PromotionGoodsData,
	products []marketing.SkcInfo,
) *marketing.CalculateSupplyPriceRequest {
	costBySKC := make(map[string]float64, len(products))
	costBySKUBySKC := make(map[string]map[string]float64, len(products))
	for _, product := range products {
		if product.Skc != "" && product.SupplyPrice > 0 {
			costBySKC[product.Skc] = product.SupplyPrice
		}
		if product.Skc == "" || len(product.SkuCostPriceInfoList) == 0 {
			continue
		}
		skuCosts := make(map[string]float64, len(product.SkuCostPriceInfoList))
		for _, skuCost := range product.SkuCostPriceInfoList {
			skuCode := normalizedPromotionSKUCode(skuCost.SkuCode)
			if skuCode == "" || skuCost.CostPrice <= 0 {
				continue
			}
			skuCosts[skuCode] = skuCost.CostPrice
			if costBySKC[product.Skc] <= 0 || skuCost.CostPrice > costBySKC[product.Skc] {
				costBySKC[product.Skc] = skuCost.CostPrice
			}
		}
		if len(skuCosts) > 0 {
			costBySKUBySKC[product.Skc] = skuCosts
		}
	}

	skcInfoList := make([]marketing.SkcPriceInfo, 0, len(goods))
	for _, item := range goods {
		activityPrice := calculatePriceByDiscount(item.USSupplyPrice, config.DiscountRate)
		costPrice := costBySKC[item.Skc]
		if strings.EqualFold(config.PriceMode, "PROFIT") {
			activityPrice = calculatePriceByProfit(item.USSupplyPrice, costPrice, config.MinProfitRate, config.FixedPriceAdjustment)
		}
		if activityPrice <= 0 {
			continue
		}

		skuInfoList := make([]marketing.SkuPriceInfo, 0, len(item.SkuInfoList))
		for _, sku := range item.SkuInfoList {
			productPrice := promotionSKUUSSupplyPrice(sku, item.USSupplyPrice)
			skuActivityPrice := activityPrice
			if strings.EqualFold(config.PriceMode, "DISCOUNT") {
				skuActivityPrice = calculatePriceByDiscount(productPrice, config.DiscountRate)
			} else if strings.EqualFold(config.PriceMode, "PROFIT") {
				if skuCostPrice := costBySKUBySKC[item.Skc][normalizedPromotionSKUCode(sku.Sku)]; skuCostPrice > 0 {
					skuActivityPrice = calculatePriceByProfit(
						productPrice,
						skuCostPrice,
						config.MinProfitRate,
						config.FixedPriceAdjustment,
					)
				} else {
					skuActivityPrice = calculateProfitModeSKUActivityPrice(
						productPrice,
						item.USSupplyPrice,
						activityPrice,
						costPrice,
						config.MinProfitRate,
						config.FixedPriceAdjustment,
					)
				}
			}
			if skuActivityPrice <= 0 {
				continue
			}
			skuInfoList = append(skuInfoList, marketing.SkuPriceInfo{
				SkuCode:       sku.Sku,
				ProductPrice:  productPrice,
				DiscountValue: skuActivityPrice,
			})
		}
		if len(skuInfoList) == 0 {
			continue
		}
		skcInfoList = append(skcInfoList, marketing.SkcPriceInfo{
			SkcName:     item.Skc,
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

func normalizedPromotionSKUCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func calculateProfitModeSKUActivityPrice(
	productPrice float64,
	baseProductPrice float64,
	baseActivityPrice float64,
	costPrice float64,
	minProfitRate float64,
	fixedAdjustment float64,
) float64 {
	if productPrice <= 0 {
		return 0
	}
	minimumPrice := calculatePriceByProfit(productPrice, costPrice, minProfitRate, fixedAdjustment)
	if baseProductPrice <= 0 || baseActivityPrice <= 0 {
		return minimumPrice
	}
	scaledPrice := productPrice * (baseActivityPrice / baseProductPrice)
	if scaledPrice > 0 && productPrice != baseProductPrice {
		return scaledPrice
	}
	if minimumPrice > 0 {
		return minimumPrice
	}
	return scaledPrice
}
