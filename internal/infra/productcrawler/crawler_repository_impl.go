// Package productcrawler 提供基于爬虫的产品仓储实现（属于 infra 层）
package productcrawler

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	amazonpkg "task-processor/internal/crawler/amazon"
	"task-processor/internal/model"
	"task-processor/internal/product"

	"github.com/sirupsen/logrus"
)

// CrawlerRepository 基于 Amazon 爬虫的产品仓储实现
type CrawlerRepository struct {
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
) product.CrawlerRepository {
	return &CrawlerRepository{
		amazonProcessor: amazonProcessor,
		amazonConfig:    amazonConfig,
		domainResolver:  domainResolver,
		logger:          logger.WithField("component", "CrawlerRepositoryImpl"),
		maxConcurrent:   5,
	}
}

// ShouldUseCrawler 判断是否应该使用爬虫
func (r *CrawlerRepository) ShouldUseCrawler(platform string) bool {
	if r.amazonConfig == nil || !r.amazonConfig.Enabled {
		return false
	}
	return r.amazonProcessor != nil && strings.EqualFold(platform, "amazon")
}

// FetchFromCrawler 使用爬虫获取产品数据
func (r *CrawlerRepository) FetchFromCrawler(ctx context.Context, req *product.FetchRequest) (*model.Product, error) {
	if r.amazonProcessor == nil {
		return nil, fmt.Errorf("Amazon爬虫未初始化")
	}

	url, zipcode, err := r.buildCrawlURL(req)
	if err != nil {
		return nil, fmt.Errorf("构建爬取URL失败: %w", err)
	}

	r.logger.Infof("开始爬取产品: URL=%s, Zipcode=%s", url, zipcode)

	p, err := r.amazonProcessor.Process(url, zipcode)
	if err != nil {
		return nil, fmt.Errorf("爬取产品失败: %w", err)
	}

	r.logger.Infof("爬取产品成功: ProductID=%s, Title=%s", req.ProductID, p.Title)
	return p, nil
}

// FetchVariantsBatch 批量获取变体数据
func (r *CrawlerRepository) FetchVariantsBatch(ctx context.Context, req *product.FetchRequest, productIDs []string) ([]*model.Product, []error) {
	if len(productIDs) == 0 {
		return nil, nil
	}

	if r.amazonProcessor == nil {
		return nil, []error{fmt.Errorf("Amazon爬虫未初始化")}
	}

	requests, err := r.buildBatchRequests(req, productIDs)
	if err != nil {
		return nil, []error{fmt.Errorf("构建批量请求失败: %w", err)}
	}

	r.logger.Infof("开始批量爬取 %d 个产品", len(requests))

	results := r.amazonProcessor.ProcessBatch(requests)

	var products []*model.Product
	var errs []error

	for i, result := range results {
		if result.Error != nil {
			r.logger.Warnf("产品 %s 爬取失败: %v", productIDs[i], result.Error)
			errs = append(errs, result.Error)
			continue
		}
		if result.Product != nil {
			products = append(products, result.Product)
		}
	}

	r.logger.Infof("批量爬取完成: 成功 %d/%d", len(products), len(productIDs))
	return products, errs
}

// GetSupportedPlatforms 获取支持的平台列表
func (r *CrawlerRepository) GetSupportedPlatforms() []string {
	if r.amazonConfig != nil && r.amazonConfig.Enabled && r.amazonProcessor != nil {
		return []string{"amazon"}
	}
	return []string{}
}

func (r *CrawlerRepository) buildCrawlURL(req *product.FetchRequest) (string, string, error) {
	domain := r.domainResolver.GetAmazonDomainByRegion(req.Region)
	if domain == "" {
		return "", "", fmt.Errorf("不支持的地区: %s", req.Region)
	}
	zipcode := r.getZipcodeForRegion(req.Region)
	url := r.domainResolver.BuildAmazonProductURL(req.Region, req.ProductID)
	return url, zipcode, nil
}

func (r *CrawlerRepository) buildBatchRequests(req *product.FetchRequest, productIDs []string) ([]model.ProductRequest, error) {
	domain := r.domainResolver.GetAmazonDomainByRegion(req.Region)
	if domain == "" {
		return nil, fmt.Errorf("不支持的地区: %s", req.Region)
	}

	zipcode := r.getZipcodeForRegion(req.Region)
	requests := make([]model.ProductRequest, 0, len(productIDs))

	for _, productID := range productIDs {
		url := r.domainResolver.BuildAmazonProductURL(req.Region, productID)
		requests = append(requests, model.ProductRequest{
			URL:     url,
			Zipcode: zipcode,
		})
	}
	return requests, nil
}

func (r *CrawlerRepository) getZipcodeForRegion(region string) string {
	if r.amazonConfig != nil && r.amazonConfig.Zipcodes != nil {
		if zipcode, exists := r.amazonConfig.Zipcodes[strings.ToLower(region)]; exists && zipcode != "" {
			return zipcode
		}
	}
	return amazonpkg.GetDefaultZipcode(strings.ToLower(region))
}

// SetMaxConcurrent 设置最大并发数
func (r *CrawlerRepository) SetMaxConcurrent(max int) {
	if max > 0 {
		r.maxConcurrent = max
	}
}
