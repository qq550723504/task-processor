// Package product 提供Amazon爬虫客户端功能
package product

import (
	"fmt"
	"strings"
	"task-processor/internal/common/amazon"
	"task-processor/internal/core/config"
	"task-processor/internal/domain/model"

	"github.com/sirupsen/logrus"
)

// CrawlerClient Amazon爬虫客户端
type CrawlerClient struct {
	amazonProcessor *amazon.AmazonProcessor
	amazonConfig    *config.AmazonConfig
	domainResolver  *DomainResolver
	logger          *logrus.Entry
}

// NewCrawlerClient 创建爬虫客户端
func NewCrawlerClient(amazonProcessor *amazon.AmazonProcessor, amazonConfig *config.AmazonConfig, logger *logrus.Entry) *CrawlerClient {
	return &CrawlerClient{
		amazonProcessor: amazonProcessor,
		amazonConfig:    amazonConfig,
		domainResolver:  NewDomainResolver(),
		logger:          logger.WithField("component", "CrawlerClient"),
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
	if c.amazonProcessor == nil {
		return nil, fmt.Errorf("Amazon爬虫未初始化")
	}

	// 根据地区获取Amazon域名和邮编
	domain := c.domainResolver.GetAmazonDomainByRegion(req.Region)
	zipcode := c.getZipcodeForRegion(req.Region)

	// 构建URL
	url := fmt.Sprintf("https://www.%s/dp/%s?th=1&psc=1&language=en_US", domain, req.ProductID)

	c.logger.Infof("🚀 开始爬取: URL=%s, Zipcode=%s", url, zipcode)

	// 调用Amazon爬虫
	product, err := c.amazonProcessor.Process(url, zipcode)
	if err != nil {
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
	defaultZipcodes := map[string]string{
		"us":                   "10001", // 纽约
		"usa":                  "10001",
		"united states":        "10001",
		"uk":                   "SW1A 1AA", // 伦敦
		"gb":                   "SW1A 1AA",
		"united kingdom":       "SW1A 1AA",
		"de":                   "10115", // 柏林
		"germany":              "10115",
		"fr":                   "75001", // 巴黎
		"france":               "75001",
		"it":                   "00118", // 罗马
		"italy":                "00118",
		"es":                   "28001", // 马德里
		"spain":                "28001",
		"ca":                   "K1A 0A6", // 渥太华
		"canada":               "K1A 0A6",
		"jp":                   "100-0001", // 东京
		"japan":                "100-0001",
		"au":                   "2000", // 悉尼
		"australia":            "2000",
		"mx":                   "01000", // 墨西哥城
		"mexico":               "01000",
		"ae":                   "00000", // 迪拜
		"uae":                  "00000",
		"united arab emirates": "00000",
		"sa":                   "11564", // 利雅得
		"saudi":                "11564",
		"saudi arabia":         "11564",
	}

	if zipcode, exists := defaultZipcodes[strings.ToLower(region)]; exists {
		return zipcode
	}

	return "10001" // 默认美国邮编
}
