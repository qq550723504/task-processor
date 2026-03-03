// Package product 提供Amazon爬虫客户端功能
package product

import (
	"fmt"
	"strings"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/domain/model"
	"task-processor/internal/pkg/utils"
	"time"

	"github.com/sirupsen/logrus"
)

// CrawlerClient Amazon爬虫客户端
type CrawlerClient struct {
	amazonProcessor  *amazon.AmazonProcessor
	processorWrapper *amazon.ProcessorWrapper
	amazonConfig     *config.AmazonConfig
	domainResolver   *DomainResolver
	logger           *logrus.Entry
}

// NewCrawlerClient 创建爬虫客户端
func NewCrawlerClient(amazonProcessor *amazon.AmazonProcessor, amazonConfig *config.AmazonConfig, logger *logrus.Entry) *CrawlerClient {
	var processorWrapper *amazon.ProcessorWrapper
	if amazonProcessor != nil {
		processorWrapper = amazon.NewProcessorWrapper(amazonProcessor)
	}

	return &CrawlerClient{
		amazonProcessor:  amazonProcessor,
		processorWrapper: processorWrapper,
		amazonConfig:     amazonConfig,
		domainResolver:   NewDomainResolver(),
		logger:           logger.WithField("component", "CrawlerClient"),
	}
}

// ShouldUseCrawler 判断是否应该使用Amazon爬虫
func (c *CrawlerClient) ShouldUseCrawler(platform string) bool {
	if c.amazonConfig == nil || !c.amazonConfig.Enabled {
		return false
	}

	if c.amazonProcessor == nil {
		return false
	}

	return strings.EqualFold(platform, "amazon")
}

// FetchFromCrawler 使用Amazon爬虫抓取数据
func (c *CrawlerClient) FetchFromCrawler(req *FetchRequest) (*model.Product, error) {
	if c.processorWrapper == nil {
		return nil, fmt.Errorf("Amazon爬虫未初始化")
	}

	// 获取邮编
	zipcode := c.getZipcodeForRegion(req.Region)

	// 构建URL(使用统一方法)
	url := c.domainResolver.BuildAmazonProductURL(req.Region, req.ProductID)

	c.logger.Infof("🚀 开始爬取: URL=%s, Zipcode=%s", url, zipcode)

	// 使用包装器调用Amazon爬虫，设置3分钟超时
	product, err := c.processorWrapper.ProcessWithTimeout(url, zipcode, 3*time.Minute)
	if err != nil {
		// 检查是否为404或页面不存在错误
		if c.isPageNotFoundError(err) {
			return nil, &model.ProductNotFoundError{
				ProductID: req.ProductID,
				Message:   err.Error(),
				Cause:     err,
			}
		}
		return nil, fmt.Errorf("抓取失败: %w", err)
	}

	return product, nil
}

// GetZipcodeForRegion 根据地区获取邮编（公开方法）
func (c *CrawlerClient) GetZipcodeForRegion(region string) string {
	return c.getZipcodeForRegion(region)
}

// getZipcodeForRegion 根据地区获取邮编
func (c *CrawlerClient) getZipcodeForRegion(region string) string {
	if c.amazonConfig == nil || c.amazonConfig.Zipcodes == nil {
		return c.getDefaultZipcode(region)
	}

	// 从配置中获取邮编
	if zipcode, exists := c.amazonConfig.Zipcodes[strings.ToLower(region)]; exists && zipcode != "" {
		return zipcode
	}

	return c.getDefaultZipcode(region)
}

// getDefaultZipcode 获取默认邮编
func (c *CrawlerClient) getDefaultZipcode(region string) string {
	// 使用统一的工具方法获取默认邮编
	return utils.GetDefaultZipcode(strings.ToLower(region))
}

// isPageNotFoundError 检查是否为页面不存在错误
func (c *CrawlerClient) isPageNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	pageNotFoundPatterns := []string{
		"页面不存在(404)",
		"页面不存在",
		"page not found",
		"404",
		"产品页面不存在",
		"产品页面缺少必要元素",
	}

	for _, pattern := range pageNotFoundPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}
