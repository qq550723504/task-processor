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
	allGoods := promotionGoodsFromProductSnapshots(products, config.Currency)
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
	return &promotionRegistrationSession{
		service:   s,
		strategy:  clonePromotionOperationStrategy(strategy),
		storeInfo: storeInfo,
	}, nil
}

type promotionRegistrationSession struct {
	service   *activityRegistrationServiceImpl
	strategy  *listingruntime.OperationStrategy
	storeInfo *listingruntime.StoreInfo
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
	allGoods := promotionGoodsFromProductSnapshots(products, config.Currency)
	return s.service.createPromotionActivityFromPreparedGoods(
		ctx,
		s.strategy,
		config,
		products,
		allGoods,
	)
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
	if strategy.ActivityStockRatio > 0 && strategy.ActivityStockRatio < 1 {
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

// promotionGoodsFromProductSnapshots builds the registration payload source from
// the locally synchronized product snapshot. The SKC supply price and the
// per-SKU customer prices have different meanings and must remain separate.
func promotionGoodsFromProductSnapshots(products []marketing.SkcInfo, _ string) []marketing.PromotionGoodsData {
	goods := make([]marketing.PromotionGoodsData, 0, len(products))
	for _, product := range products {
		skc := strings.TrimSpace(product.Skc)
		if skc == "" || product.Stock <= 0 {
			continue
		}
		supplyPrice := product.SupplyPrice
		skuInfoList := make([]marketing.PromotionSkuInfo, 0, len(product.SkuPriceInfoList))
		for _, skuPrice := range product.SkuPriceInfoList {
			skuCode := strings.TrimSpace(skuPrice.SkuCode)
			if skuCode == "" {
				continue
			}
			skuInfoList = append(skuInfoList, marketing.PromotionSkuInfo{Sku: skuCode})
		}
		if len(skuInfoList) == 0 {
			continue
		}
		goods = append(goods, marketing.PromotionGoodsData{
			Skc:              skc,
			InventoryNum:     product.Stock,
			USSupplyPrice:    supplyPrice,
			MaxUSSupplyPrice: supplyPrice,
			SkuInfoList:      skuInfoList,
		})
	}
	return goods
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
		if item.InventoryNum <= 0 && product.Stock > 0 {
			item.InventoryNum = product.Stock
		}
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

func promotionFloat64Ptr(value float64) *float64 {
	return &value
}

type promotionSKUPriceInput struct {
	SKU         string
	RetailPrice float64
	CostPrice   float64
	Currency    string
}

func promotionSKUPriceInputs(products []marketing.SkcInfo, currency string) map[string]map[string]promotionSKUPriceInput {
	targetCurrency := configCurrencyOrDefault(currency)
	inputsBySKC := make(map[string]map[string]promotionSKUPriceInput, len(products))
	for _, product := range products {
		skc := strings.TrimSpace(product.Skc)
		if skc == "" {
			continue
		}
		inputsBySKU := inputsBySKC[skc]
		if inputsBySKU == nil {
			inputsBySKU = make(map[string]promotionSKUPriceInput)
			inputsBySKC[skc] = inputsBySKU
		}
		for _, skuSitePrice := range product.SkuPriceInfoList {
			sku := normalizedPromotionSKUCode(skuSitePrice.SkuCode)
			if sku == "" {
				continue
			}
			input := inputsBySKU[sku]
			input.SKU = sku
			input.Currency = targetCurrency
			for _, sitePrice := range skuSitePrice.SitePriceInfoList {
				if input.RetailPrice == 0 && sitePrice.IsAvailable && sitePrice.SalePrice > 0 && strings.EqualFold(sitePrice.Currency, targetCurrency) {
					input.RetailPrice = sitePrice.SalePrice
				}
			}
			inputsBySKU[sku] = input
		}
		for _, skuCost := range product.SkuCostPriceInfoList {
			sku := normalizedPromotionSKUCode(skuCost.SkuCode)
			if sku == "" || skuCost.CostPrice <= 0 || !strings.EqualFold(skuCost.Currency, targetCurrency) {
				continue
			}
			input := inputsBySKU[sku]
			input.SKU = sku
			input.Currency = targetCurrency
			if input.CostPrice == 0 {
				input.CostPrice = skuCost.CostPrice
			}
			inputsBySKU[sku] = input
		}
	}
	return inputsBySKC
}

func (s *activityRegistrationServiceImpl) buildCalculateRequestForPromotionProducts(
	config TimeLimitedDiscountConfig,
	goods []marketing.PromotionGoodsData,
	products []marketing.SkcInfo,
) *marketing.CalculateSupplyPriceRequest {
	mode := strings.ToUpper(strings.TrimSpace(config.PriceMode))
	priceInputsBySKC := promotionSKUPriceInputs(products, config.Currency)
	skcInfoList := make([]marketing.SkcPriceInfo, 0, len(goods))
	for _, item := range goods {
		priceInputsBySKU := priceInputsBySKC[strings.TrimSpace(item.Skc)]
		skuInfoList := make([]marketing.SkuPriceInfo, 0, len(item.SkuInfoList))
		for _, sku := range item.SkuInfoList {
			input, ok := priceInputsBySKU[normalizedPromotionSKUCode(sku.Sku)]
			if !ok || input.RetailPrice <= 0 {
				continue
			}
			productPrice := input.RetailPrice
			var skuActivityPrice float64
			switch mode {
			case "PROFIT":
				if input.CostPrice <= 0 {
					continue
				}
				skuActivityPrice = calculatePriceByProfit(productPrice, input.CostPrice, config.MinProfitRate, config.FixedPriceAdjustment)
			case "BREAKEVEN":
				if input.CostPrice <= 0 {
					continue
				}
				skuActivityPrice = calculatePriceByBreakeven(productPrice, input.CostPrice, config.FixedPriceAdjustment)
			default:
				skuActivityPrice = calculatePriceByDiscount(productPrice, config.DiscountRate)
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
