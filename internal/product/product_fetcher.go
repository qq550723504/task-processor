// Package product 提供产品领域服务
package product

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/core/config"
	corelogger "task-processor/internal/core/logger"
	"task-processor/internal/model"
	"task-processor/internal/ports"
	"task-processor/internal/product/sourcing"

	"github.com/sirupsen/logrus"
)

// ProductFetcher 产品获取器
type ProductFetcher struct {
	cacheManager  *CacheManager
	amazonConfig  *config.AmazonConfig
	sourceFetcher sourcing.AmazonSourceFetcher
	logger        *logrus.Entry
}

// NewProductFetcher 创建产品获取器
func NewProductFetcher(
	rawJsonDataClient RawJsonDataClient,
	amazonConfig *config.AmazonConfig,
	crawlSource ports.CrawlSource,
) *ProductFetcher {
	return NewProductFetcherWithLogger(rawJsonDataClient, amazonConfig, crawlSource, nil)
}

// NewProductFetcherWithLogger 创建产品获取器，支持传入自定义 logger。
func NewProductFetcherWithLogger(
	rawJsonDataClient RawJsonDataClient,
	amazonConfig *config.AmazonConfig,
	crawlSource ports.CrawlSource,
	log *logrus.Entry,
) *ProductFetcher {
	if log == nil {
		log = corelogger.GetGlobalLogger("product.fetcher")
	}
	zipcodes := map[string]string(nil)
	if amazonConfig != nil {
		zipcodes = amazonConfig.Zipcodes
	}
	return &ProductFetcher{
		cacheManager: NewCacheManagerWithFreshness(rawJsonDataClient, log, amazonConfigFreshnessDays(amazonConfig)),
		amazonConfig: amazonConfig,
		sourceFetcher: sourcing.AmazonSourceFetcher{
			Planner: sourcing.AmazonCrawlRequestPlanner{
				DomainResolver: sourcing.AmazonDefaultDomainResolver{},
				ZipcodePolicy:  sourcing.AmazonDefaultZipcodePolicy{},
				Zipcodes:       zipcodes,
			},
			Source: crawlSource,
		},
		logger: log,
	}
}

func amazonConfigFreshnessDays(cfg *config.AmazonConfig) int {
	if cfg == nil {
		return 0
	}
	return cfg.DataFreshnessDays
}

// FetchProduct 获取产品
func (f *ProductFetcher) FetchProduct(ctx context.Context, req *FetchRequest) (*model.Product, error) {
	if f.cacheManager != nil && f.shouldUseCache(req) {
		if product, err := f.cacheManager.GetFromCache(req); err == nil {
			f.logger.Debugf("got product from cache: %s", req.ProductID)
			return product, nil
		}
	}

	if f.sourceFetcher.Configured() && f.amazonConfig != nil && f.amazonConfig.Enabled {
		f.logger.Debugf("fetching product via crawler: %s", req.ProductID)
		return f.sourceFetcher.Fetch(ctx, sourcing.AmazonCrawlRequestInput{
			Region:    req.Region,
			ProductID: req.ProductID,
			Zipcode:   req.Zipcode,
		})
	}

	return nil, f.crawlerUnavailableError(req)
}

// FetchProductWithRetry 带重试的产品获取
func (f *ProductFetcher) FetchProductWithRetry(productID, region string, storeID int64, maxRetries int) (*model.Product, error) {
	req := &FetchRequest{ProductID: productID, Region: region, StoreID: storeID}
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		product, err := f.FetchProduct(context.Background(), req)
		if err == nil {
			return product, nil
		}
		lastErr = err
		f.logger.Warnf("retry %d fetch product failed: %v", i+1, err)
	}
	return nil, lastErr
}

// CacheProduct 缓存产品数据到服务端
func (f *ProductFetcher) CacheProduct(req *FetchRequest, product *model.Product) error {
	if !f.shouldUseCache(req) {
		f.logger.Debug("skip cache because request uses explicit zipcode")
		return nil
	}
	if product == nil {
		f.logger.Warn("product is nil, skipping cache")
		return nil
	}
	if f.cacheManager == nil {
		f.logger.Warn("cacheManager is nil, cannot cache product")
		return nil
	}
	return f.cacheManager.CacheProduct(req, product)
}

// CacheVariants 批量缓存变体数据到服务端
func (f *ProductFetcher) CacheVariants(req *FetchRequest, variants []*model.Product) error {
	if len(variants) == 0 {
		return nil
	}
	if f.cacheManager == nil {
		f.logger.Warn("cacheManager is nil, cannot cache variants")
		return nil
	}
	return f.cacheManager.CacheVariants(req, variants)
}

// FetchVariants 批量获取变体数据
func (f *ProductFetcher) FetchVariants(ctx context.Context, req *FetchRequest, variantASINs []string) ([]*model.Product, error) {
	if len(variantASINs) == 0 {
		return []*model.Product{}, nil
	}

	variants := make([]*model.Product, 0, len(variantASINs))
	for _, asin := range variantASINs {
		variantReq := VariantFetchRequest(req, asin)
		product, err := f.FetchProduct(ctx, variantReq)
		if err != nil {
			f.logger.Warnf("fetch variant failed: ASIN=%s, err=%v", asin, err)
			continue
		}
		if product != nil {
			if strings.TrimSpace(product.Asin) != "" && product.Asin != asin {
				f.logger.Infof("variant fetch redirected ASIN: requested=%s actual=%s; preserving requested ASIN for downstream mapping", asin, product.Asin)
				normalized := *product
				normalized.Asin = asin
				product = &normalized
			}
			if cacheErr := f.CacheProduct(variantReq, product); cacheErr != nil {
				f.logger.Warnf("cache variant immediately failed: ASIN=%s, err=%v", asin, cacheErr)
			}
			variants = append(variants, product)
		}
	}
	return variants, nil
}

// GetStats 获取统计信息
func (f *ProductFetcher) GetStats() map[string]any {
	return map[string]any{"type": "local"}
}

func (f *ProductFetcher) shouldUseCache(req *FetchRequest) bool {
	return req == nil || strings.TrimSpace(req.Zipcode) == ""
}

func (f *ProductFetcher) crawlerUnavailableError(req *FetchRequest) error {
	productID := ""
	region := ""
	if req != nil {
		productID = req.ProductID
		region = req.Region
	}

	switch {
	case !f.sourceFetcher.Configured():
		return fmt.Errorf("crawler source is not configured for product fetch: product_id=%s region=%s", productID, region)
	case f.amazonConfig == nil || !f.amazonConfig.Enabled:
		return fmt.Errorf("amazon crawler is disabled for product fetch: product_id=%s region=%s", productID, region)
	default:
		return fmt.Errorf("crawler fetch is unavailable for product fetch: product_id=%s region=%s", productID, region)
	}
}
