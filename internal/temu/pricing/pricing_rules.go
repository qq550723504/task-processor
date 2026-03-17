package pricing

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"task-processor/internal/model"
	"task-processor/internal/product"
	temupricing "task-processor/internal/temu/api/pricing"

	"github.com/sirupsen/logrus"
)

// getFromCache 从缓存获取数据
func (s *PricingDecisionService) getFromCache(key CacheKey) (any, bool) {
	if item, ok := s.cache.Load(key); ok {
		cacheItem := item.(CacheItem)
		if time.Now().Before(cacheItem.ExpiresAt) {
			return cacheItem.Data, true
		}
		s.cache.Delete(key)
	}
	return nil, false
}

// setCache 设置缓存
func (s *PricingDecisionService) setCache(key CacheKey, data any) {
	item := CacheItem{
		Data:      data,
		ExpiresAt: time.Now().Add(s.config.CacheTimeout),
	}
	s.cache.Store(key, item)
}

// getAmazonProductWithCache 获取Amazon产品数据（带缓存）
func (s *PricingDecisionService) getAmazonProductWithCache(ctx context.Context, productID, region string, tenantID, storeID int64) (*model.Product, error) {
	if s.productFetcher == nil {
		return nil, errors.New("ProductFetcher未初始化，无法获取Amazon产品数据")
	}

	cacheKey := CacheKey{
		Type:      "amazon_product",
		ProductID: fmt.Sprintf("%s_%s_%d_%d", productID, region, tenantID, storeID),
		StoreID:   storeID,
	}

	if cached, found := s.getFromCache(cacheKey); found {
		if p, ok := cached.(*model.Product); ok {
			s.logger.Debugf("从缓存获取Amazon产品数据: %s", productID)
			return p, nil
		}
	}

	req := &product.FetchRequest{
		TenantID:  tenantID,
		Platform:  "Amazon",
		Region:    region,
		ProductID: productID,
		StoreID:   storeID,
	}

	var amazonProduct *model.Product
	var lastErr error

	for attempt := 1; attempt <= s.config.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		amazonProduct, lastErr = s.productFetcher.FetchProduct(req)
		if lastErr == nil {
			s.logger.Debugf("第%d次尝试成功获取Amazon产品数据: %s", attempt, productID)
			s.setCache(cacheKey, amazonProduct)
			return amazonProduct, nil
		}

		s.logger.Warnf("第%d次获取Amazon产品数据失败: %v", attempt, lastErr)
		if attempt < s.config.MaxRetries {
			s.logger.Infof("将进行第%d次重试获取Amazon产品数据: %s", attempt+1, productID)
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	return nil, fmt.Errorf("经过%d次重试后仍无法获取Amazon产品数据: %w", s.config.MaxRetries, lastErr)
}

// makeDecisionByPrice 根据价格做出决策
func (s *PricingDecisionService) makeDecisionByPrice(actualPrice, minAcceptablePrice float64) *temupricing.Decision {
	decision := &temupricing.Decision{}

	if actualPrice >= minAcceptablePrice {
		decision.Action = temupricing.DecisionAccept
		decision.Reason = fmt.Sprintf("价格%.2f >= 最低可接受价%.2f，满足要求",
			actualPrice, minAcceptablePrice)
		return decision
	}

	strategy := s.storeConfig.GetPriceRejectStrategy()
	if strategy == "TAKE_OFFLINE" {
		decision.Action = temupricing.DecisionReject
		decision.Reason = fmt.Sprintf("价格%.2f < 最低可接受价%.2f，根据店铺配置执行下架",
			actualPrice, minAcceptablePrice)
	} else if s.storeConfig.IsRebargainEnabled() {
		decision.Action = temupricing.DecisionReappeal
		decision.Reason = fmt.Sprintf("价格%.2f < 最低可接受价%.2f，根据店铺配置保留在售并重新报价",
			actualPrice, minAcceptablePrice)
	} else {
		decision.Action = temupricing.DecisionSkip
		decision.Reason = fmt.Sprintf("价格%.2f < 最低可接受价%.2f，店铺未启用重新议价，保留在售",
			actualPrice, minAcceptablePrice)
	}

	return decision
}

// logPricingInfo 记录定价信息
func (s *PricingDecisionService) logPricingInfo(pricingCtx *PricingContext) {
	s.logger.WithFields(logrus.Fields{
		"goods_name":           pricingCtx.GoodsName,
		"sku_sn":               pricingCtx.SkuSN,
		"origin_cost_price":    pricingCtx.OriginCostPrice,
		"supplier_price":       pricingCtx.SupplierPrice,
		"min_acceptable_price": pricingCtx.MinAcceptablePrice,
	}).Info("定价信息")
}

// logSalesBoostPricingInfo 记录销量提升定价信息
func (s *PricingDecisionService) logSalesBoostPricingInfo(goods *temupricing.SalesBoostGoods, sku *temupricing.SalesBoostSku, pricingCtx *PricingContext, targetPrice, profitMargin float64) {
	s.logger.WithFields(logrus.Fields{
		"goods_id":             goods.SalesBoostGoodsBasicInfo.GoodsID,
		"sku_sn":               sku.OutSkuSN,
		"origin_cost_price":    pricingCtx.OriginCostPrice,
		"current_price":        pricingCtx.SupplierPrice,
		"target_price":         targetPrice,
		"min_acceptable_price": pricingCtx.MinAcceptablePrice,
		"profit_margin":        profitMargin,
	}).Info("销量提升定价信息")
}

// parsePrice 解析价格字符串为浮点数
func parsePrice(price string) float64 {
	if price == "" {
		return 0.0
	}
	result, err := strconv.ParseFloat(price, 64)
	if err != nil {
		fmt.Sscanf(price, "%f", &result)
	}
	return result
}
