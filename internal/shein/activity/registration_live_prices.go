package activity

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/listingruntime"
	"task-processor/internal/shein/api/marketing"
)

// withLivePromotionSKUPrices replaces candidate snapshot sale prices with the
// current per-SKU supply prices returned by SHEIN's promotion-goods endpoint.
func (s *activityRegistrationServiceImpl) withLivePromotionSKUPrices(
	ctx context.Context,
	strategy *listingruntime.OperationStrategy,
	products []marketing.SkcInfo,
) ([]marketing.SkcInfo, map[string]string, error) {
	livePriceProducts := make([]marketing.SkcInfo, 0, len(products))
	for _, product := range products {
		if product.UseLivePromotionSKUPrices {
			livePriceProducts = append(livePriceProducts, product)
		}
	}
	if len(livePriceProducts) == 0 {
		return products, nil, nil
	}
	if strategy == nil {
		return nil, nil, fmt.Errorf("operation strategy is required")
	}

	storeInfo, err := s.getStoreInfo(ctx, strategy.StoreID)
	if err != nil {
		return nil, nil, fmt.Errorf("获取店铺信息失败: %w", err)
	}
	config := s.buildPromotionCreateConfig(storeInfo, strategy, "PROMOTION", livePriceProducts)
	goods, err := s.queryAllPromotionGoods(config)
	if err != nil {
		return nil, nil, fmt.Errorf("查询促销活动商品SKU价格失败: %w", err)
	}

	goodsBySKC := make(map[string]marketing.PromotionGoodsData, len(goods))
	for _, item := range goods {
		if skc := strings.TrimSpace(item.Skc); skc != "" {
			goodsBySKC[skc] = item
		}
	}
	updated := make([]marketing.SkcInfo, 0, len(products))
	filterReasons := make(map[string]string)
	for _, product := range products {
		if !product.UseLivePromotionSKUPrices {
			updated = append(updated, product)
			continue
		}
		skc := product.Skc
		liveGoods, ok := goodsBySKC[strings.TrimSpace(skc)]
		if !ok {
			filterReasons[skc] = "未查询到活动商品实时SKU供货价"
			continue
		}
		product, ok = promotionProductWithLiveSKUPrices(product, liveGoods)
		if !ok {
			filterReasons[skc] = "活动商品实时SKU供货价不完整"
			continue
		}
		updated = append(updated, product)
	}
	if len(filterReasons) == 0 {
		filterReasons = nil
	}
	return updated, filterReasons, nil
}

func promotionProductWithLiveSKUPrices(product marketing.SkcInfo, goods marketing.PromotionGoodsData) (marketing.SkcInfo, bool) {
	if len(product.SkuPriceInfoList) == 0 || len(goods.SkuInfoList) == 0 {
		return marketing.SkcInfo{}, false
	}
	livePriceBySKU := make(map[string]float64, len(goods.SkuInfoList))
	for _, sku := range goods.SkuInfoList {
		skuCode := normalizePromotionSKUCode(sku.Sku)
		if skuCode == "" || sku.USSupplyPrice == nil || *sku.USSupplyPrice <= 0 {
			continue
		}
		livePriceBySKU[skuCode] = *sku.USSupplyPrice
	}
	if len(livePriceBySKU) != len(product.SkuPriceInfoList) {
		return marketing.SkcInfo{}, false
	}

	updatedSKUPrices := make([]marketing.SkuSitePriceInfo, 0, len(product.SkuPriceInfoList))
	seen := make(map[string]struct{}, len(product.SkuPriceInfoList))
	for _, skuPrice := range product.SkuPriceInfoList {
		skuCode := normalizePromotionSKUCode(skuPrice.SkuCode)
		price, ok := livePriceBySKU[skuCode]
		if skuCode == "" || !ok || price <= 0 {
			return marketing.SkcInfo{}, false
		}
		if _, duplicate := seen[skuCode]; duplicate {
			return marketing.SkcInfo{}, false
		}
		seen[skuCode] = struct{}{}
		updatedSKUPrices = append(updatedSKUPrices, marketing.SkuSitePriceInfo{
			SkuCode: skuPrice.SkuCode,
			SitePriceInfoList: []marketing.SitePriceInfo{{
				SiteCode: "US", SalePrice: price, Currency: "USD", IsAvailable: true,
			}},
		})
	}

	product.SkuPriceInfoList = updatedSKUPrices
	product.UseLivePromotionSKUPrices = false
	if goods.USSupplyPrice > 0 {
		product.SitePriceInfoList = []marketing.SitePriceInfo{{
			SiteCode: "US", SalePrice: goods.USSupplyPrice, Currency: "USD", IsAvailable: true,
		}}
	}
	return product, true
}
