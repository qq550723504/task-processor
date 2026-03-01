// Package product 提供分布式产品数据获取功能
package product

import (
	"context"
	"fmt"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/domain/model"
	"task-processor/internal/infra/crawler"

	"github.com/sirupsen/logrus"
)

// DistributedProductFetcher 分布式产品数据获取器
// 使用分布式爬虫集群替代本地Amazon处理器
type DistributedProductFetcher struct {
	rawJsonDataClient  RawJsonDataClient
	distributedCrawler *crawler.DistributedCrawlerClient
	amazonConfig       *config.AmazonConfig
	logger             *logrus.Entry

	// 专职处理器
	cacheManager   *CacheManager
	dataParser     *DataParser
	domainResolver *DomainResolver
}

// NewDistributedProductFetcher 创建分布式产品数据获取器
func NewDistributedProductFetcher(
	rawJsonDataClient RawJsonDataClient,
	amazonConfig *config.AmazonConfig,
	rabbitmqURL string,
) (*DistributedProductFetcher, error) {
	logger := logrus.WithField("component", "DistributedProductFetcher")

	// 创建分布式爬虫客户端
	distributedCrawler, err := crawler.NewDistributedCrawlerClient(rabbitmqURL, logger.Logger)
	if err != nil {
		return nil, fmt.Errorf("创建分布式爬虫客户端失败: %w", err)
	}

	// 设置超时时间
	if amazonConfig != nil && amazonConfig.CrawlTimeout > 0 {
		distributedCrawler.SetTimeout(time.Duration(amazonConfig.CrawlTimeout) * time.Second)
	}

	// 创建专职处理器
	cacheManager := NewCacheManager(rawJsonDataClient, logger)
	dataParser := NewDataParser(logger)
	domainResolver := NewDomainResolver()

	return &DistributedProductFetcher{
		rawJsonDataClient:  rawJsonDataClient,
		distributedCrawler: distributedCrawler,
		amazonConfig:       amazonConfig,
		logger:             logger,
		cacheManager:       cacheManager,
		dataParser:         dataParser,
		domainResolver:     domainResolver,
	}, nil
}

// FetchProduct 获取产品数据（使用分布式爬虫）
func (f *DistributedProductFetcher) FetchProduct(req *FetchRequest) (*model.Product, error) {
	f.logger.Infof("🔍 开始获取产品数据: ProductID=%s, Platform=%s, Region=%s",
		req.ProductID, req.Platform, req.Region)

	// 第一步：检查缓存
	product, err := f.cacheManager.GetFromCache(req)
	if err == nil && product != nil {
		f.logger.Infof("✅ 从缓存获取产品数据: ProductID=%s", req.ProductID)
		return product, nil
	}

	if err != nil {
		f.logger.Debugf("缓存获取失败或数据需要更新: %v", err)
	}

	// 第二步：使用分布式爬虫获取
	if f.shouldUseCrawler(req.Platform) {
		f.logger.Infof("🌐 使用分布式爬虫抓取: ProductID=%s", req.ProductID)

		product, err := f.fetchFromDistributedCrawler(req)
		if err != nil {
			return nil, fmt.Errorf("分布式爬虫抓取失败: %w", err)
		}

		// 保存到缓存
		if saveErr := f.cacheManager.SaveToCache(req, product); saveErr != nil {
			f.logger.Warnf("⚠️ 保存到缓存失败: %v", saveErr)
		}

		f.logger.Infof("✅ 分布式爬虫抓取成功: ProductID=%s", req.ProductID)
		return product, nil
	}

	return nil, fmt.Errorf("无法获取产品数据: ProductID=%s, Platform=%s", req.ProductID, req.Platform)
}

// fetchFromDistributedCrawler 从分布式爬虫获取产品数据
func (f *DistributedProductFetcher) fetchFromDistributedCrawler(req *FetchRequest) (*model.Product, error) {
	// 构建爬虫请求
	crawlReq := &crawler.CrawlRequest{
		TaskID:    time.Now().UnixNano(), // 生成唯一任务ID
		TenantID:  req.TenantID,
		StoreID:   req.StoreID,
		Platform:  req.Platform,
		Region:    req.Region,
		ProductID: req.ProductID,
		URL:       f.buildProductURL(req),
		Zipcode:   f.getZipcode(req.Region),
		Priority:  f.calculatePriority(req),
	}

	// 提交爬虫任务并等待结果
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := f.distributedCrawler.SubmitCrawlTask(ctx, crawlReq)
	if err != nil {
		return nil, fmt.Errorf("提交分布式爬虫任务失败: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("分布式爬虫任务失败: %s", result.Error)
	}

	if result.Product == nil {
		return nil, fmt.Errorf("分布式爬虫返回空产品数据")
	}

	f.logger.Infof("🎉 分布式爬虫任务完成: TaskID=%d, Duration=%v, NodeID=%s",
		result.TaskID, result.Duration, result.NodeID)

	return result.Product, nil
}

// buildProductURL 构建产品URL
func (f *DistributedProductFetcher) buildProductURL(req *FetchRequest) string {
	if req.Platform == "amazon" {
		domain := f.domainResolver.GetAmazonDomainByRegion(req.Region)
		return fmt.Sprintf("https://%s/dp/%s", domain, req.ProductID)
	}
	return ""
}

// getZipcode 获取区域对应的邮编
func (f *DistributedProductFetcher) getZipcode(region string) string {
	zipcodes := map[string]string{
		"us": "10001",    // 纽约
		"uk": "SW1A 1AA", // 伦敦
		"de": "10115",    // 柏林
		"fr": "75001",    // 巴黎
		"it": "00118",    // 罗马
		"es": "28001",    // 马德里
		"ca": "K1A 0A6",  // 渥太华
		"jp": "100-0001", // 东京
		"au": "2000",     // 悉尼
	}

	if zipcode, exists := zipcodes[region]; exists {
		return zipcode
	}
	return "10001" // 默认美国邮编
}

// calculatePriority 计算任务优先级
func (f *DistributedProductFetcher) calculatePriority(req *FetchRequest) int {
	// 基础优先级
	priority := 5

	// 根据平台调整
	if req.Platform == "amazon" {
		priority += 2
	}

	// 根据分类调整（热门分类优先级更高）
	if req.CategoryID > 0 && req.CategoryID < 1000 {
		priority += 1
	}

	// 确保在1-10范围内
	if priority > 10 {
		priority = 10
	}
	if priority < 1 {
		priority = 1
	}

	return priority
}

// shouldUseCrawler 判断是否应该使用爬虫
func (f *DistributedProductFetcher) shouldUseCrawler(platform string) bool {
	// 目前只支持Amazon平台的爬虫
	return platform == "amazon"
}

// CacheProduct 缓存产品数据到服务器
func (f *DistributedProductFetcher) CacheProduct(req *FetchRequest, product *model.Product) error {
	if product == nil {
		f.logger.Warn("产品数据为空，跳过缓存")
		return nil
	}

	f.logger.Infof("💾 开始缓存产品数据到服务器: ProductID=%s", req.ProductID)
	return f.cacheManager.CacheProduct(req, product)
}

// CacheVariants 批量缓存变体数据到服务器
func (f *DistributedProductFetcher) CacheVariants(req *FetchRequest, variants []*model.Product) error {
	if len(variants) == 0 {
		f.logger.Debug("没有变体数据，跳过缓存")
		return nil
	}

	f.logger.Infof("💾 开始批量缓存变体数据到服务器: 数量=%d", len(variants))
	return f.cacheManager.CacheVariants(req, variants)
}

// FetchVariants 获取变体数据（批量处理）
func (f *DistributedProductFetcher) FetchVariants(req *FetchRequest, variantASINs []string) ([]*model.Product, error) {
	if len(variantASINs) == 0 {
		return []*model.Product{}, nil
	}

	f.logger.Infof("🔍 开始批量获取变体数据: 数量=%d", len(variantASINs))

	var variants []*model.Product
	var errors []error

	// 并发获取变体数据
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

		variant, err := f.FetchProduct(variantReq)
		if err != nil {
			f.logger.Warnf("获取变体失败: ASIN=%s, Error=%v", asin, err)
			errors = append(errors, err)
			continue
		}

		variants = append(variants, variant)
	}

	f.logger.Infof("✅ 变体数据获取完成: 成功=%d, 失败=%d", len(variants), len(errors))

	if len(variants) == 0 && len(errors) > 0 {
		return nil, fmt.Errorf("所有变体获取失败: %d个错误", len(errors))
	}

	return variants, nil
}

// GetStats 获取统计信息
func (f *DistributedProductFetcher) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"type": "distributed",
	}

	if f.distributedCrawler != nil {
		crawlerStats := f.distributedCrawler.GetStats()
		stats["crawler"] = crawlerStats
	}

	return stats
}

// Close 关闭获取器
func (f *DistributedProductFetcher) Close() error {
	f.logger.Info("关闭分布式产品数据获取器")

	if f.distributedCrawler != nil {
		return f.distributedCrawler.Close()
	}

	return nil
}
