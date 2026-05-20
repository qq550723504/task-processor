// Package product 提供产品领域服务
package product

import (
	"context"
	"strings"

	"task-processor/internal/core/config"
	corelogger "task-processor/internal/core/logger"
	"task-processor/internal/model"
	"task-processor/internal/ports"

	"github.com/sirupsen/logrus"
)

// ProductFetcher 产品获取器
type ProductFetcher struct {
	cacheManager *CacheManager
	amazonConfig *config.AmazonConfig
	crawlSource  ports.CrawlSource
	logger       *logrus.Entry
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
	return &ProductFetcher{
		cacheManager: NewCacheManager(rawJsonDataClient, log),
		amazonConfig: amazonConfig,
		crawlSource:  crawlSource,
		logger:       log,
	}
}

// FetchProduct 获取产品
func (f *ProductFetcher) FetchProduct(ctx context.Context, req *FetchRequest) (*model.Product, error) {
	if f.cacheManager != nil && f.shouldUseCache(req) {
		if product, err := f.cacheManager.GetFromCache(req); err == nil {
			f.logger.Debugf("got product from cache: %s", req.ProductID)
			return product, nil
		}
	}

	if f.crawlSource != nil && f.amazonConfig != nil && f.amazonConfig.Enabled {
		f.logger.Debugf("fetching product via crawler: %s", req.ProductID)
		resolver := NewDomainResolver()
		productURL := resolver.BuildAmazonProductURL(req.Region, req.ProductID)
		zipcode := strings.TrimSpace(req.Zipcode)
		if zipcode == "" && resolver.ShouldUseDefaultZipcode(req.Region) {
			zipcode = resolver.GetZipcodeByRegion(req.Region)
		}
		return f.crawlSource.ProcessWithContext(ctx, productURL, zipcode)
	}

	return nil, nil
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
		variantReq := &FetchRequest{
			TenantID:   req.TenantID,
			Platform:   req.Platform,
			Region:     req.Region,
			ProductID:  asin,
			StoreID:    req.StoreID,
			CategoryID: req.CategoryID,
			Creator:    req.Creator,
		}
		product, err := f.FetchProduct(ctx, variantReq)
		if err != nil {
			f.logger.Warnf("fetch variant failed: ASIN=%s, err=%v", asin, err)
			continue
		}
		if product != nil {
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
