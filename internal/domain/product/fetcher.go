// Package product 提供产品数据获取相关功能
package product

import (
	"fmt"
	"task-processor/internal/common/amazon"
	"task-processor/internal/core/config"
	"task-processor/internal/domain/model"
	"task-processor/internal/pkg/management/api"

	"github.com/sirupsen/logrus"
)

// ProductFetcher 产品数据获取器（支持从API或Amazon爬虫获取）
type ProductFetcher struct {
	rawJsonDataClient RawJsonDataClient
	amazonProcessor   *amazon.AmazonProcessor
	amazonConfig      *config.AmazonConfig
	logger            *logrus.Entry

	// 注入的专职处理器
	cacheManager   *CacheManager
	crawlerClient  *CrawlerClient
	dataParser     *DataParser
	domainResolver *DomainResolver
}

// RawJsonDataClient 原始JSON数据客户端接口
type RawJsonDataClient interface {
	GetRawJsonData(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error)
	CreateRawJsonData(req *api.RawJsonDataCreateReqDTO) (int64, error)
}

// FetchRequest 获取请求
type FetchRequest struct {
	TenantID   int64
	Platform   string
	Region     string
	ProductID  string
	StoreID    int64
	CategoryID int64
	Creator    string
}

// NewProductFetcher 创建产品数据获取器
func NewProductFetcher(
	rawJsonDataClient RawJsonDataClient,
	amazonConfig *config.AmazonConfig,
	amazonProcessor *amazon.AmazonProcessor,
) *ProductFetcher {
	logger := logrus.WithField("component", "ProductFetcher")

	// 创建专职处理器
	cacheManager := NewCacheManager(rawJsonDataClient, logger)
	crawlerClient := NewCrawlerClient(amazonProcessor, amazonConfig, logger)
	dataParser := NewDataParser(logger)
	domainResolver := NewDomainResolver()

	return &ProductFetcher{
		rawJsonDataClient: rawJsonDataClient,
		amazonProcessor:   amazonProcessor,
		amazonConfig:      amazonConfig,
		logger:            logger,
		cacheManager:      cacheManager,
		crawlerClient:     crawlerClient,
		dataParser:        dataParser,
		domainResolver:    domainResolver,
	}
}

// FetchProduct 获取产品数据（优先从API，如果没有则从Amazon爬虫）
func (f *ProductFetcher) FetchProduct(req *FetchRequest) (*model.Product, error) {
	f.logger.Infof("🔍 开始获取产品数据: ProductID=%s, Platform=%s, Region=%s",
		req.ProductID, req.Platform, req.Region)

	// 第一步：检查缓存
	product, err := f.cacheManager.GetFromCache(req)
	if err == nil && product != nil {
		return product, nil
	}

	if err != nil {
		f.logger.Debugf("缓存获取失败或数据需要更新: %v", err)
	}

	// 第二步：使用爬虫获取
	if f.crawlerClient.ShouldUseCrawler(req.Platform) {
		f.logger.Infof("🌐 使用爬虫抓取: ProductID=%s", req.ProductID)

		product, err := f.crawlerClient.FetchFromCrawler(req)
		if err != nil {
			return nil, fmt.Errorf("爬虫抓取失败: %w", err)
		}

		// 保存到缓存
		if saveErr := f.cacheManager.SaveToCache(req, product); saveErr != nil {
			f.logger.Warnf("⚠️ 保存到缓存失败: %v", saveErr)
		}

		return product, nil
	}

	return nil, fmt.Errorf("无法获取产品数据: ProductID=%s, Platform=%s", req.ProductID, req.Platform)
}

// CacheProduct 缓存产品数据到服务器（用于已获取的产品数据）
func (f *ProductFetcher) CacheProduct(req *FetchRequest, product *model.Product) error {
	if product == nil {
		f.logger.Warn("产品数据为空，跳过缓存")
		return nil
	}

	f.logger.Infof("💾 开始缓存产品数据到服务器: ProductID=%s", req.ProductID)
	return f.cacheManager.CacheProduct(req, product)
}

// CacheVariants 批量缓存变体数据到服务器
func (f *ProductFetcher) CacheVariants(req *FetchRequest, variants []*model.Product) error {
	if len(variants) == 0 {
		f.logger.Debug("没有变体数据，跳过缓存")
		return nil
	}

	f.logger.Infof("💾 开始批量缓存变体数据到服务器: 数量=%d", len(variants))
	return f.cacheManager.CacheVariants(req, variants)
}
