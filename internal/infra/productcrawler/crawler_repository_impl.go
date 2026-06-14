// Package productcrawler 提供基于爬虫的产品仓储实现（属于 infra 层）
package productcrawler

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/core/config"
	crawleramazon "task-processor/internal/integration/crawler/amazon"
	"task-processor/internal/model"
	"task-processor/internal/product"
	"task-processor/internal/product/sourcing"

	"github.com/sirupsen/logrus"
)

// CrawlerRepository 基于 Amazon 爬虫的产品仓储实现
type CrawlerRepository struct {
	amazonConfig  *config.AmazonConfig
	sourceFetcher sourcing.AmazonSourceFetcher
	logger        *logrus.Entry
	maxConcurrent int
}

// DomainResolver 域名解析器接口
type DomainResolver interface {
	GetAmazonDomainByRegion(region string) string
	BuildAmazonProductURL(region, asin string) string
}

// NewCrawlerRepositoryImpl 创建爬虫仓储实现
func NewCrawlerRepositoryImpl(
	amazonProcessor crawleramazon.Source,
	amazonConfig *config.AmazonConfig,
	domainResolver DomainResolver,
	logger *logrus.Entry,
) product.CrawlerRepository {
	zipcodes := map[string]string(nil)
	if amazonConfig != nil {
		zipcodes = amazonConfig.Zipcodes
	}
	amazonCrawler := crawleramazon.NewProcessor(amazonProcessor)
	return &CrawlerRepository{
		amazonConfig: amazonConfig,
		sourceFetcher: sourcing.AmazonSourceFetcher{
			Planner: sourcing.AmazonCrawlRequestPlanner{
				DomainResolver: domainResolver,
				ZipcodePolicy:  crawleramazon.ZipcodePolicy{},
				Zipcodes:       zipcodes,
			},
			Source: amazonCrawler,
		},
		logger:        logger.WithField("component", "CrawlerRepositoryImpl"),
		maxConcurrent: 5,
	}
}

// ShouldUseCrawler 判断是否应该使用爬虫
func (r *CrawlerRepository) ShouldUseCrawler(platform string) bool {
	if r.amazonConfig == nil || !r.amazonConfig.Enabled {
		return false
	}
	return r.sourceFetcher.Configured() && strings.EqualFold(platform, "amazon")
}

// FetchFromCrawler 使用爬虫获取产品数据
func (r *CrawlerRepository) FetchFromCrawler(ctx context.Context, req *product.FetchRequest) (*model.Product, error) {
	if !r.sourceFetcher.Configured() {
		return nil, fmt.Errorf("Amazon爬虫未初始化")
	}

	input := sourcing.AmazonCrawlRequestInput{
		Region:    req.Region,
		ProductID: req.ProductID,
		Zipcode:   req.Zipcode,
	}

	r.logger.Infof("开始爬取产品: ProductID=%s", req.ProductID)

	p, err := r.sourceFetcher.Fetch(ctx, input)
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

	if !r.sourceFetcher.Configured() {
		return nil, []error{fmt.Errorf("Amazon爬虫未初始化")}
	}

	results, err := r.sourceFetcher.FetchBatch(ctx, sourcing.AmazonCrawlRequestInput{
		Region:  req.Region,
		Zipcode: req.Zipcode,
	}, productIDs)
	if err != nil {
		return nil, []error{fmt.Errorf("构建批量请求失败: %w", err)}
	}

	r.logger.Infof("开始批量爬取 %d 个产品", len(productIDs))

	var products []*model.Product
	var errs []error

	for _, result := range sourcing.NormalizeAmazonBatchResults(sourcing.AmazonCrawlRequestInput{
		Region:  req.Region,
		Zipcode: req.Zipcode,
	}, productIDs, results) {
		if result.Error != nil {
			r.logger.Warnf("产品 %s 爬取失败: %v", result.Identity.ProductID, result.Error)
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
	if r.amazonConfig != nil && r.amazonConfig.Enabled && r.sourceFetcher.Configured() {
		return []string{"amazon"}
	}
	return []string{}
}

// SetMaxConcurrent 设置最大并发数
func (r *CrawlerRepository) SetMaxConcurrent(max int) {
	if max > 0 {
		r.maxConcurrent = max
	}
}
