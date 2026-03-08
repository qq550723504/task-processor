// Package messaging 提供爬虫处理器注册功能
package messaging

import (
	"fmt"

	"task-processor/internal/app/processor"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/domain/product"
	"task-processor/internal/infra/auth"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/pkg/management"

	"github.com/sirupsen/logrus"
)

// CrawlerRegistry 爬虫处理器注册器
type CrawlerRegistry struct {
	config         *config.Config
	logger         *logrus.Logger
	rabbitmqClient *rabbitmq.Client
}

// NewCrawlerRegistry 创建爬虫处理器注册器
func NewCrawlerRegistry(
	cfg *config.Config,
	logger *logrus.Logger,
	rabbitmqClient *rabbitmq.Client,
) *CrawlerRegistry {
	return &CrawlerRegistry{
		config:         cfg,
		logger:         logger,
		rabbitmqClient: rabbitmqClient,
	}
}

// RegisterCrawlerProcessor 注册爬虫处理器到服务管理器
func (r *CrawlerRegistry) RegisterCrawlerProcessor(serviceManager *ServiceManager) error {
	return r.RegisterCrawlerProcessorWithAmazon(serviceManager, nil)
}

// RegisterCrawlerProcessorWithAmazon 注册爬虫处理器到服务管理器（可选共享Amazon处理器）
func (r *CrawlerRegistry) RegisterCrawlerProcessorWithAmazon(serviceManager *ServiceManager, sharedAmazonProcessor *amazon.AmazonProcessor) error {
	r.logger.Info("📦 注册Amazon爬虫处理器...")

	// 使用共享的Amazon处理器，如果没有则创建新的
	var amazonProcessor *amazon.AmazonProcessor
	if sharedAmazonProcessor != nil {
		r.logger.Info("✅ 复用共享的Amazon处理器（避免重复初始化浏览器池）")
		amazonProcessor = sharedAmazonProcessor
	} else {
		r.logger.Info("🔧 创建新的Amazon处理器")
		amazonProcessor = amazon.CreateProcessor(r.config, r.logger)
	}

	// 创建产品获取器
	productFetcher := r.createProductFetcher(amazonProcessor)
	if productFetcher == nil {
		return fmt.Errorf("创建产品获取器失败")
	}

	// 创建任务提交器
	taskSubmitter := NewTaskSubmitter(r.rabbitmqClient, r.logger)

	// 创建爬虫处理器
	crawlerProcessor := processor.NewCrawlerProcessor(
		r.logger,
		amazonProcessor,
		productFetcher,
		taskSubmitter,
	)

	// 注册到服务管理器（使用 amazon.crawler 避免与上架服务冲突）
	if err := serviceManager.RegisterProcessor("amazon.crawler", crawlerProcessor); err != nil {
		return fmt.Errorf("注册Amazon爬虫处理器失败: %w", err)
	}

	r.logger.Info("✅ Amazon爬虫处理器注册成功")
	return nil
}

// createProductFetcher 创建产品获取器
func (r *CrawlerRegistry) createProductFetcher(amazonProcessor *amazon.AmazonProcessor) *product.ProductFetcher {
	// 创建认证客户端
	authClient := auth.NewClientCredentialsAuthClient(
		r.config.Management.BaseURL,
		r.config.Management.ClientID,
		r.config.Management.ClientSecret,
		r.config.Management.TenantID,
		r.logger,
	)

	// 创建管理客户端
	managementClient := management.NewClientManager(&r.config.Management)

	// 先获取一次客户端，确保客户端已创建
	_ = managementClient.GetClient()

	// 获取访问令牌并设置
	token, err := authClient.GetAccessToken()
	if err != nil {
		r.logger.Warnf("⚠️ 获取访问令牌失败: %v (将以无认证模式运行)", err)
		return nil
	}

	// 设置访问令牌
	managementClient.SetUserToken(token, r.config.Management.TenantID)
	r.logger.Info("✅ 访问令牌设置成功")

	// 设置数据新鲜度
	managementClient.SetDataFreshnessDays(r.config.Amazon.DataFreshnessDays)

	// 创建产品获取器
	productFetcher := product.NewProductFetcher(
		managementClient.GetRawJsonDataClient(),
		&r.config.Amazon,
		amazonProcessor,
	)

	return productFetcher
}
