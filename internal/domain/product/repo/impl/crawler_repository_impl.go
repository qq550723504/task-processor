// Package impl 提供爬虫仓储的具体实现
package impl

import (
	"context"
	"fmt"
	"strings"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product/repo"
	"task-processor/internal/domain/product/types"
	"task-processor/internal/pkg/utils"

	"github.com/sirupsen/logrus"
)

// CrawlerRepositoryImpl 爬虫仓储实现
type CrawlerRepositoryImpl struct {
	amazonProcessor *amazon.AmazonProcessor
	amazonConfig    *config.AmazonConfig
	domainResolver  DomainResolver
	logger          *logrus.Entry
	maxConcurrent   int
}

// DomainResolver 域名解析器接口
type DomainResolver interface {
	GetAmazonDomainByRegion(region string) string
	BuildAmazonProductURL(region, asin string) string
}

// NewCrawlerRepositoryImpl 创建爬虫仓储实现
func NewCrawlerRepositoryImpl(
	amazonProcessor *amazon.AmazonProcessor,
	amazonConfig *config.AmazonConfig,
	domainResolver DomainResolver,
	logger *logrus.Entry,
) repo.CrawlerRepository {
	return &CrawlerRepositoryImpl{
		amazonProcessor: amazonProcessor,
		amazonConfig:    amazonConfig,
		domainResolver:  domainResolver,
		logger:          logger.WithField("component", "CrawlerRepositoryImpl"),
		maxConcurrent:   5, // 默认并发数
	}
}

// ShouldUseCrawler 判断是否应该使用爬虫
func (r *CrawlerRepositoryImpl) ShouldUseCrawler(platform string) bool {
	if r.amazonConfig == nil || !r.amazonConfig.Enabled {
		return false
	}

	if r.amazonProcessor == nil {
		return false
	}

	return strings.EqualFold(platform, "amazon")
}

// FetchFromCrawler 使用爬虫获取产品数据
func (r *CrawlerRepositoryImpl) FetchFromCrawler(ctx context.Context, req *types.FetchRequest) (*model.Product, error) {
	if r.amazonProcessor == nil {
		return nil, fmt.Errorf("Amazon爬虫未初始化")
	}

	// 构建爬取URL
	url, zipcode, err := r.buildCrawlURL(req)
	if err != nil {
		return nil, fmt.Errorf("构建爬取URL失败: %w", err)
	}

	r.logger.Infof("开始爬取产品: URL=%s, Zipcode=%s", url, zipcode)

	// 执行爬取
	product, err := r.amazonProcessor.Process(url, zipcode)
	if err != nil {
		return nil, fmt.Errorf("爬取产品失败: %w", err)
	}

	r.logger.Infof("爬取产品成功: ProductID=%s, Title=%s", req.ProductID, product.Title)
	return product, nil
}

// FetchVariantsBatch 批量获取变体数据
func (r *CrawlerRepositoryImpl) FetchVariantsBatch(ctx context.Context, req *types.FetchRequest, productIDs []string) ([]*model.Product, []error) {
	if len(productIDs) == 0 {
		return nil, nil
	}

	if r.amazonProcessor == nil {
		return nil, []error{fmt.Errorf("Amazon爬虫未初始化")}
	}

	// 构建批量请求
	requests, err := r.buildBatchRequests(req, productIDs)
	if err != nil {
		return nil, []error{fmt.Errorf("构建批量请求失败: %w", err)}
	}

	r.logger.Infof("开始批量爬取 %d 个产品", len(requests))

	// 执行批量爬取
	results := r.amazonProcessor.ProcessBatch(requests)

	// 处理结果
	var products []*model.Product
	var errors []error

	for i, result := range results {
		if result.Error != nil {
			r.logger.Warnf("产品 %s 爬取失败: %v", productIDs[i], result.Error)
			errors = append(errors, result.Error)
			continue
		}

		if result.Product != nil {
			products = append(products, result.Product)
		}
	}

	r.logger.Infof("批量爬取完成: 成功 %d/%d", len(products), len(productIDs))
	return products, errors
}

// GetSupportedPlatforms 获取支持的平台列表
func (r *CrawlerRepositoryImpl) GetSupportedPlatforms() []string {
	platforms := []string{}

	if r.amazonConfig != nil && r.amazonConfig.Enabled && r.amazonProcessor != nil {
		platforms = append(platforms, "amazon")
	}

	return platforms
}

// buildCrawlURL 构建爬取URL
func (r *CrawlerRepositoryImpl) buildCrawlURL(req *types.FetchRequest) (string, string, error) {
	// 获取域名
	domain := r.domainResolver.GetAmazonDomainByRegion(req.Region)
	if domain == "" {
		return "", "", fmt.Errorf("不支持的地区: %s", req.Region)
	}

	// 获取邮编
	zipcode := r.getZipcodeForRegion(req.Region)

	// 构建URL(使用统一方法)
	url := r.domainResolver.BuildAmazonProductURL(req.Region, req.ProductID)

	return url, zipcode, nil
}

// buildBatchRequests 构建批量请求
func (r *CrawlerRepositoryImpl) buildBatchRequests(req *types.FetchRequest, productIDs []string) ([]model.ProductRequest, error) {
	domain := r.domainResolver.GetAmazonDomainByRegion(req.Region)
	if domain == "" {
		return nil, fmt.Errorf("不支持的地区: %s", req.Region)
	}

	zipcode := r.getZipcodeForRegion(req.Region)
	requests := make([]model.ProductRequest, 0, len(productIDs))

	for _, productID := range productIDs {
		// 使用统一的URL构建方法
		url := r.domainResolver.BuildAmazonProductURL(req.Region, productID)
		requests = append(requests, model.ProductRequest{
			URL:     url,
			Zipcode: zipcode,
		})
	}

	return requests, nil
}

// getZipcodeForRegion 根据地区获取邮编
func (r *CrawlerRepositoryImpl) getZipcodeForRegion(region string) string {
	// 优先从配置获取
	if r.amazonConfig != nil && r.amazonConfig.Zipcodes != nil {
		if zipcode, exists := r.amazonConfig.Zipcodes[strings.ToLower(region)]; exists && zipcode != "" {
			return zipcode
		}
	}

	// 使用统一的工具方法获取默认邮编
	return utils.GetDefaultZipcode(strings.ToLower(region))
}

// SetMaxConcurrent 设置最大并发数
func (r *CrawlerRepositoryImpl) SetMaxConcurrent(max int) {
	if max > 0 {
		r.maxConcurrent = max
	}
}
