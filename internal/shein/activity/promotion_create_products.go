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

	config := s.buildTimeLimitedDiscountConfig(storeInfo, strategy)
	applyPromotionCreateConfig(&config, strategy)
	config.FilterSkcList = selectedPromotionSKCs(products)
	if validateErr := config.Validate(); validateErr != nil {
		return nil, fmt.Errorf("配置验证失败: %w", validateErr)
	}

	allGoods, err := s.queryAllPromotionGoods(config)
	if err != nil {
		return nil, fmt.Errorf("查询商品失败: %w", err)
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

	createReq, filterReasons, filterReasonBySKC := s.buildCreateActivityRequest(config, goods, calcResp)
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
			"activity_key": activityKey,
			"activity_id":  createResp.Info.ActivityID,
			"skc_count":    len(createReq.AddCostAndStockInfoList),
		}).Info("成功创建 SHEIN 促销活动")
	}
	return result, nil
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
		if item.USSupplyPrice <= 0 && fallbackPrice > 0 {
			item.USSupplyPrice = fallbackPrice
		}
		if item.MaxUSSupplyPrice <= 0 && item.USSupplyPrice > 0 {
			item.MaxUSSupplyPrice = item.USSupplyPrice
		}
		if item.InventoryNum <= 0 && product.Stock > 0 {
			item.InventoryNum = product.Stock
		}
		enriched = append(enriched, item)
	}
	return enriched
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

func (s *activityRegistrationServiceImpl) buildCalculateRequestForPromotionProducts(
	config TimeLimitedDiscountConfig,
	goods []marketing.PromotionGoodsData,
	products []marketing.SkcInfo,
) *marketing.CalculateSupplyPriceRequest {
	costBySKC := make(map[string]float64, len(products))
	for _, product := range products {
		if product.Skc != "" && product.SupplyPrice > 0 {
			costBySKC[product.Skc] = product.SupplyPrice
		}
	}

	skcInfoList := make([]marketing.SkcPriceInfo, 0, len(goods))
	for _, item := range goods {
		activityPrice := calculatePriceByDiscount(item.USSupplyPrice, config.DiscountRate)
		if strings.EqualFold(config.PriceMode, "PROFIT") {
			costPrice := costBySKC[item.Skc]
			activityPrice = calculatePriceByProfit(item.USSupplyPrice, costPrice, config.MinProfitRate, config.FixedPriceAdjustment)
		}
		if activityPrice <= 0 {
			continue
		}

		skuInfoList := make([]marketing.SkuPriceInfo, 0, len(item.SkuInfoList))
		for _, sku := range item.SkuInfoList {
			skuInfoList = append(skuInfoList, marketing.SkuPriceInfo{
				SkuCode:       sku.Sku,
				ProductPrice:  item.USSupplyPrice,
				DiscountValue: activityPrice,
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
