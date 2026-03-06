// Package main 提供爬虫RabbitMQ消费者主程序
package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product"
	"task-processor/internal/infra/auth"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/pkg/management"
	"task-processor/internal/pkg/utils"

	"github.com/sirupsen/logrus"
)

var (
	configPath = flag.String("config", "config/rabbitmq-config.yaml", "RabbitMQ配置文件路径")
	appConfig  = flag.String("app-config", "config/config-dev.yaml", "应用配置文件路径")
	logLevel   = flag.String("log-level", "info", "日志级别")
)

// 版本信息
var (
	appVersion = "1.0.0"
	buildTime  = "unknown"
)

func main() {
	flag.Parse()

	// 设置日志
	logger := utils.SetupLoggerWithLevel(*logLevel)

	// 打印版本信息
	utils.PrintVersionInfo(logger, utils.VersionInfo{
		Version:   appVersion,
		BuildTime: buildTime,
	})

	logger.Info("🚀 启动Amazon爬虫消费者...")

	// 加载应用配置
	appCfg := config.LoadConfigWithFallback(*appConfig, logger)

	// 创建服务管理器
	serviceManager, err := rabbitmq.NewServiceManager(*configPath, logger)
	if err != nil {
		logger.Fatalf("❌ 创建服务管理器失败: %v", err)
	}

	// 创建并注册爬虫处理器
	if err := registerCrawlerProcessor(serviceManager, appCfg, logger); err != nil {
		logger.Fatalf("❌ 注册爬虫处理器失败: %v", err)
	}

	// 启动服务
	ctx := context.Background()
	if err := serviceManager.Start(ctx); err != nil {
		logger.Fatalf("❌ 启动服务失败: %v", err)
	}

	logger.Info("✅ Amazon爬虫消费者启动完成")
	logger.Info("📊 监控地址:")
	logger.Info("   - 健康检查: http://localhost:8081/health")
	logger.Info("   - 就绪检查: http://localhost:8081/ready")
	logger.Info("   - 指标监控: http://localhost:8082/metrics")
	logger.Info("   - 统计信息: http://localhost:8082/stats")
	logger.Info("🔄 按 Ctrl+C 优雅关闭服务")

	// 等待服务管理器完成
	serviceManager.Wait()
	logger.Info("程序已退出")
}

// registerCrawlerProcessor 注册爬虫处理器
func registerCrawlerProcessor(serviceManager *rabbitmq.ServiceManager, appCfg *config.Config, logger *logrus.Logger) error {
	logger.Info("📦 注册Amazon爬虫处理器...")

	// 创建共享的Amazon处理器
	amazonProcessor := createAmazonProcessor(appCfg, logger)

	// 获取RabbitMQ客户端（用于提交变体任务）
	rabbitmqClient := serviceManager.GetClient()

	// 创建爬虫处理器
	crawlerProcessor := NewCrawlerProcessor(appCfg, logger, amazonProcessor, rabbitmqClient)

	// 注册到服务管理器
	if err := serviceManager.RegisterProcessor("amazon", crawlerProcessor); err != nil {
		return fmt.Errorf("注册Amazon爬虫处理器失败: %w", err)
	}

	logger.Info("✅ Amazon爬虫处理器注册成功")
	return nil
}

// createAmazonProcessor 创建Amazon处理器
func createAmazonProcessor(cfg *config.Config, logger *logrus.Logger) *amazon.AmazonProcessor {
	logger.Info("🔧 创建Amazon爬虫处理器...")

	// 确保浏览器配置合理
	if cfg.Browser.PoolSize <= 0 {
		cfg.Browser.PoolSize = 3 // 爬虫默认使用3个浏览器实例
	}

	// 创建Amazon爬虫处理器
	amazonProcessor := amazon.NewAmazonProcessor(cfg)

	logger.Infof("✅ Amazon爬虫处理器创建成功，浏览器池大小: %d", cfg.Browser.PoolSize)
	return amazonProcessor
}

// CrawlerProcessor Amazon爬虫处理器
type CrawlerProcessor struct {
	amazonProcessor *amazon.AmazonProcessor
	productFetcher  *product.ProductFetcher
	taskSubmitter   *rabbitmq.TaskSubmitter
	logger          *logrus.Logger
	config          *config.Config
}

// NewCrawlerProcessor 创建爬虫处理器
func NewCrawlerProcessor(
	cfg *config.Config,
	logger *logrus.Logger,
	amazonProcessor *amazon.AmazonProcessor,
	rabbitmqClient *rabbitmq.Client,
) *CrawlerProcessor {
	// 创建认证客户端
	authClient := auth.NewClientCredentialsAuthClient(
		cfg.Management.BaseURL,
		cfg.Management.ClientID,
		cfg.Management.ClientSecret,
		cfg.Management.TenantID,
		logger,
	)

	// 创建管理客户端
	managementClient := management.NewClientManager(&cfg.Management)

	// 先获取一次客户端，确保客户端已创建
	_ = managementClient.GetClient()

	// 获取访问令牌并设置
	token, err := authClient.GetAccessToken()
	if err != nil {
		logger.Warnf("⚠️ 获取访问令牌失败: %v (将以无认证模式运行)", err)
		return nil
	} else {
		// 设置访问令牌
		managementClient.SetUserToken(token, cfg.Management.TenantID)
		logger.Info("✅ 访问令牌设置成功")
	}

	// 设置数据新鲜度
	managementClient.SetDataFreshnessDays(cfg.Amazon.DataFreshnessDays)

	// 创建产品获取器
	productFetcher := product.NewProductFetcher(
		managementClient.GetRawJsonDataClient(),
		&cfg.Amazon,
		amazonProcessor,
	)

	// 创建任务提交器
	taskSubmitter := rabbitmq.NewTaskSubmitter(rabbitmqClient, logger)

	return &CrawlerProcessor{
		amazonProcessor: amazonProcessor,
		productFetcher:  productFetcher,
		taskSubmitter:   taskSubmitter,
		logger:          logger,
		config:          cfg,
	}
}

// Start 启动处理器
func (p *CrawlerProcessor) Start(ctx context.Context) error {
	p.logger.Info("🌐 Amazon爬虫处理器启动完成")
	return nil
}

// ProcessTask 处理任务 - 实现worker.Processor接口
func (p *CrawlerProcessor) ProcessTask(ctx context.Context, task *model.Task) error {
	p.logger.Infof("🔍 开始爬取任务: ID=%d, ProductID=%s", task.ID, task.ProductID)

	startTime := time.Now()

	// 构建获取请求
	fetchReq := &product.FetchRequest{
		TenantID:   task.TenantID,
		Platform:   task.Platform,
		Region:     task.Region,
		ProductID:  task.ProductID,
		StoreID:    task.StoreID,
		CategoryID: task.CategoryID,
		Creator:    "crawler-consumer",
	}

	// 获取产品数据（会自动使用浏览器池，浏览器实例会被放回池中复用）
	productData, err := p.productFetcher.FetchProduct(fetchReq)
	if err != nil {
		p.logger.Errorf("❌ 爬取失败: ID=%d, ProductID=%s, Error=%v", task.ID, task.ProductID, err)
		return fmt.Errorf("爬取产品数据失败: %w", err)
	}

	// 打印产品基本信息
	p.logger.Infof("📦 产品ASIN: %s", productData.Asin)
	p.logger.Infof("💰 产品价格: %.2f %s", productData.FinalPrice, productData.Currency)

	// 保存产品数据到服务器（如果服务器已有数据则跳过）
	if err := p.productFetcher.CacheProduct(fetchReq, productData); err != nil {
		p.logger.Warnf("⚠️ 保存产品数据到服务器失败: %v", err)
		// 不返回错误，因为数据已经获取成功
	}

	// 只有主产品任务才提交变体任务（避免无限递归）
	// 通过Remark字段判断：如果Remark为"variant"，说明这是变体任务，不再提交变体
	if task.Remark != "variant" && len(productData.Variations) > 0 {
		p.logger.Infof("🔄 发现 %d 个变体，准备提交爬虫任务", len(productData.Variations))
		successCount, failCount := p.taskSubmitter.SubmitVariantTasks(ctx, task, productData.Variations, productData.ParentAsin)
		p.logger.Infof("📤 变体任务提交完成: 成功=%d, 失败=%d, 总数=%d",
			successCount, failCount, len(productData.Variations))
	} else if task.Remark == "variant" {
		p.logger.Debugf("这是变体任务，跳过变体提交（避免递归）")
	}

	duration := time.Since(startTime)
	p.logger.Infof("✅ 爬取完成: ID=%d, ProductID=%s, 耗时=%v", task.ID, task.ProductID, duration)

	return nil
}

// Close 关闭处理器
func (p *CrawlerProcessor) Close(ctx context.Context) {
	p.logger.Info("🔒 关闭Amazon爬虫处理器")
	// 注意：不要在这里关闭amazonProcessor，因为它是共享的
	// amazonProcessor会在main函数退出时由serviceManager统一关闭
}

// GetStatus 获取处理器状态
func (p *CrawlerProcessor) GetStatus() map[string]any {
	return map[string]any{
		"name":   "Amazon爬虫处理器",
		"status": "running",
	}
}
