// Package service 提供业务逻辑层
package service

import (
	"context"
	"fmt"
	"log"

	"task-processor/internal/common/amazon"
	"task-processor/internal/repo"
	"task-processor/internal/utils"
)

// CrawlerService 爬虫服务
type CrawlerService struct {
	configService *ConfigService
	fileRepo      *repo.FileRepository
	urlBuilder    *utils.URLBuilder
}

// NewCrawlerService 创建爬虫服务实例
func NewCrawlerService(
	configService *ConfigService,
	fileRepo *repo.FileRepository,
	urlBuilder *utils.URLBuilder,
) *CrawlerService {
	return &CrawlerService{
		configService: configService,
		fileRepo:      fileRepo,
		urlBuilder:    urlBuilder,
	}
}

// CrawlerRequest 爬虫请求参数
type CrawlerRequest struct {
	URL        string
	Zipcode    string
	Region     string
	Output     string
	ConfigFile string
}

// ProcessProduct 处理产品爬取
func (s *CrawlerService) ProcessProduct(ctx context.Context, req *CrawlerRequest) error {
	// 处理URL和邮编
	url, zipcode := s.processURLAndZipcode(req)

	// 加载配置
	cfg := s.configService.LoadConfig(req.ConfigFile)

	// 创建处理器
	processor := amazon.NewAmazonProcessor(&cfg.Amazon)
	defer processor.Shutdown()

	// 处理页面
	log.Printf("开始处理Amazon产品: %s", url)
	product, err := processor.Process(url, zipcode)
	if err != nil {
		return fmt.Errorf("处理页面失败: %w", err)
	}

	// 保存结果
	if err := s.fileRepo.SaveProduct(product, req.Output); err != nil {
		return fmt.Errorf("保存文件失败: %w", err)
	}

	log.Printf("成功保存结果到: %s", req.Output)
	log.Printf("产品标题: %s", product.Title)
	log.Printf("产品价格: %.2f %s", product.FinalPrice, product.Currency)

	return nil
}

// processURLAndZipcode 处理URL和邮编
func (s *CrawlerService) processURLAndZipcode(req *CrawlerRequest) (string, string) {
	url := req.URL
	zipcode := req.Zipcode

	// 如果没有提供URL，构建默认URL
	if url == "" {
		url = s.urlBuilder.BuildDefaultURL(req.Region)
	}

	// 如果没有提供邮编，使用默认邮编
	if zipcode == "" {
		zipcode = utils.GetDefaultZipcode(req.Region)
	}

	return url, zipcode
}
