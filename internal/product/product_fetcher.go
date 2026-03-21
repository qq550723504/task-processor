// Package product 提供产品领域服务
package product

import (
	"context"
	"task-processor/internal/core/config"
	corelogger "task-processor/internal/core/logger"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

// AmazonScraper 定义 Amazon 商品抓取能力（消费者定义接口原则）。
// 与 crawler/amazon.Scraper 保持相同签名，domain 层不直接依赖 crawler 包。
type AmazonScraper interface {
	Process(url string, zipcode string) (*model.Product, error)
	ProcessWithContext(ctx context.Context, url string, zipcode string) (*model.Product, error)
}

// ProductFetcher 产品获取器
type ProductFetcher struct {
	cacheManager    *CacheManager
	amazonConfig    *config.AmazonConfig
	amazonProcessor AmazonScraper
	logger          *logrus.Entry
}

// NewProductFetcher 创建产品获取器
func NewProductFetcher(
	rawJsonDataClient RawJsonDataClient,
	amazonConfig *config.AmazonConfig,
	amazonProcessor AmazonScraper,
) *ProductFetcher {
	return NewProductFetcherWithLogger(rawJsonDataClient, amazonConfig, amazonProcessor, nil)
}

// NewProductFetcherWithLogger 创建产品获取器，支持传入自定义 logger。
// logger 为 nil 时使用全局日志管理器。
func NewProductFetcherWithLogger(
	rawJsonDataClient RawJsonDataClient,
	amazonConfig *config.AmazonConfig,
	amazonProcessor AmazonScraper,
	log *logrus.Entry,
) *ProductFetcher {
	if log == nil {
		log = corelogger.GetGlobalLogger("product.fetcher")
	}
	return &ProductFetcher{
		cacheManager:    NewCacheManager(rawJsonDataClient, log),
		amazonConfig:    amazonConfig,
		amazonProcessor: amazonProcessor,
		logger:          log,
	}
}

// FetchProduct 获取产品
func (f *ProductFetcher) FetchProduct(ctx context.Context, req *FetchRequest) (*model.Product, error) {
	if f.cacheManager != nil {
		if product, err := f.cacheManager.GetFromCache(req); err == nil {
			f.logger.Debugf("从缓存获取产品成功: %s", req.ProductID)
			return product, nil
		}
	}

	if f.amazonProcessor != nil && f.amazonConfig != nil && f.amazonConfig.Enabled {
		f.logger.Debugf("使用爬虫获取产品: %s", req.ProductID)
		resolver := NewDomainResolver()
		productURL := resolver.BuildAmazonProductURL(req.Region, req.ProductID)
		zipcode := resolver.GetZipcodeByRegion(req.Region)
		return f.amazonProcessor.ProcessWithContext(ctx, productURL, zipcode)
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
		f.logger.Warnf("第%d次尝试获取产品失败: %v", i+1, err)
	}
	return nil, lastErr
}

// CacheProduct 缓存产品数据到服务器
func (f *ProductFetcher) CacheProduct(req *FetchRequest, product *model.Product) error {
	if product == nil {
		f.logger.Warn("产品数据为空，跳过缓存")
		return nil
	}
	if f.cacheManager == nil {
		f.logger.Warn("cacheManager未初始化，无法缓存")
		return nil
	}
	return f.cacheManager.CacheProduct(req, product)
}

// CacheVariants 批量缓存变体数据到服务器
func (f *ProductFetcher) CacheVariants(req *FetchRequest, variants []*model.Product) error {
	if len(variants) == 0 {
		return nil
	}
	if f.cacheManager == nil {
		f.logger.Warn("cacheManager未初始化，无法缓存变体")
		return nil
	}
	return f.cacheManager.CacheVariants(req, variants)
}

// GetStats 获取统计信息
func (f *ProductFetcher) GetStats() map[string]any {
	return map[string]any{"type": "local"}
}
