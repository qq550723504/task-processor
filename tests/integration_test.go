// Package tests 提供分布式爬虫系统集成测试
package tests

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"task-processor/internal/app/messaging"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/crawler"
	"task-processor/internal/infra/rabbitmq"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfig 测试配置
type TestConfig struct {
	RabbitMQURL string
	Timeout     time.Duration
}

// IntegrationTestSuite 集成测试套件
type IntegrationTestSuite struct {
	config         *TestConfig
	logger         *logrus.Logger
	crawlerClient  *crawler.DistributedCrawlerClient
	serviceManager *messaging.ServiceManager
}

// NewIntegrationTestSuite 创建集成测试套件
func NewIntegrationTestSuite() *IntegrationTestSuite {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	config := &TestConfig{
		RabbitMQURL: getEnvOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		Timeout:     30 * time.Second,
	}

	return &IntegrationTestSuite{
		config: config,
		logger: logger,
	}
}

// SetupSuite 设置测试套件
func (suite *IntegrationTestSuite) SetupSuite(t *testing.T) {
	suite.logger.Info("🚀 开始设置集成测试环境")

	// 创建分布式爬虫客户端
	crawlerClient, err := crawler.NewDistributedCrawlerClient(suite.config.RabbitMQURL, suite.logger)
	require.NoError(t, err, "创建爬虫客户端失败")
	suite.crawlerClient = crawlerClient

	// 加载配置文件
	cfg := config.LoadConfigFromFile("config/config-dev.yaml")
	require.NotNil(t, cfg, "加载配置文件失败")
	require.NotNil(t, cfg.RabbitMQ, "RabbitMQ配置为空")

	// 创建服务管理器（模拟爬虫节点）
	serviceManager, err := messaging.NewServiceManager(cfg.RabbitMQ, suite.logger)
	require.NoError(t, err, "创建服务管理器失败")
	suite.serviceManager = serviceManager

	suite.logger.Info("✅ 集成测试环境设置完成")
}

// TearDownSuite 清理测试套件
func (suite *IntegrationTestSuite) TearDownSuite(t *testing.T) {
	suite.logger.Info("🧹 开始清理集成测试环境")

	if suite.crawlerClient != nil {
		suite.crawlerClient.Close()
	}

	if suite.serviceManager != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		suite.serviceManager.Stop(ctx)
	}

	suite.logger.Info("✅ 集成测试环境清理完成")
}

// TestDistributedCrawlerEndToEnd 端到端测试
func TestDistributedCrawlerEndToEnd(t *testing.T) {
	// 检查是否应该运行集成测试
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("跳过集成测试。设置 INTEGRATION_TEST=true 环境变量来运行")
	}

	suite := NewIntegrationTestSuite()
	suite.SetupSuite(t)
	defer suite.TearDownSuite(t)

	t.Run("测试Amazon产品爬取", func(t *testing.T) {
		suite.testAmazonCrawling(t)
	})

	t.Run("测试并发爬取", func(t *testing.T) {
		suite.testConcurrentCrawling(t)
	})

	t.Run("测试错误处理", func(t *testing.T) {
		suite.testErrorHandling(t)
	})

	t.Run("测试超时处理", func(t *testing.T) {
		suite.testTimeoutHandling(t)
	})
}

// testAmazonCrawling 测试Amazon产品爬取
func (suite *IntegrationTestSuite) testAmazonCrawling(t *testing.T) {
	suite.logger.Info("🧪 开始测试Amazon产品爬取")

	// 创建爬虫请求
	request := &crawler.CrawlRequest{
		TaskID:    12345,
		TenantID:  1,
		StoreID:   617,
		Platform:  "amazon",
		Region:    "us",
		ProductID: "B08BKKQ2NL", // Echo Dot 4th Gen - 测试用产品
		URL:       "https://www.amazon.com/dp/B08BKKQ2NL",
		Zipcode:   "10001",
		Priority:  1,
	}

	// 提交爬虫任务
	ctx, cancel := context.WithTimeout(context.Background(), suite.config.Timeout)
	defer cancel()

	suite.logger.Infof("📤 提交爬虫任务: ProductID=%s", request.ProductID)
	result, err := suite.crawlerClient.SubmitCrawlTask(ctx, request)

	// 验证结果
	if err != nil {
		// 如果是因为没有运行爬虫节点导致的超时，跳过测试
		if ctx.Err() == context.DeadlineExceeded {
			t.Skip("⏰ 爬虫节点未运行，跳过测试")
			return
		}
		t.Fatalf("❌ 爬虫任务失败: %v", err)
	}

	assert.NotNil(t, result, "爬虫结果不应为空")
	assert.Equal(t, request.TaskID, result.TaskID, "任务ID应该匹配")

	if result.Success {
		assert.NotNil(t, result.Product, "成功时产品数据不应为空")
		assert.NotEmpty(t, result.NodeID, "节点ID不应为空")
		suite.logger.Infof("✅ 爬虫成功: 产品=%s, 节点=%s, 耗时=%v",
			result.Product.Title, result.NodeID, result.Duration)
	} else {
		assert.NotEmpty(t, result.Error, "失败时错误信息不应为空")
		suite.logger.Warnf("⚠️ 爬虫失败: %s", result.Error)
	}
}

// testConcurrentCrawling 测试并发爬取
func (suite *IntegrationTestSuite) testConcurrentCrawling(t *testing.T) {
	suite.logger.Info("🧪 开始测试并发爬取")

	// 测试产品列表
	testProducts := []string{
		"B08BKKQ2NL", // Echo Dot 4th Gen
		"B07XJ8C8F5", // Echo Dot 3rd Gen
		"B08KGG7ZY3", // Echo Show 8
	}

	// 并发提交任务
	results := make(chan *crawler.CrawlResult, len(testProducts))
	errors := make(chan error, len(testProducts))

	ctx, cancel := context.WithTimeout(context.Background(), suite.config.Timeout)
	defer cancel()

	for i, productID := range testProducts {
		go func(taskID int, productID string) {
			request := &crawler.CrawlRequest{
				TaskID:    int64(taskID),
				TenantID:  1,
				StoreID:   617,
				Platform:  "amazon",
				Region:    "us",
				ProductID: productID,
				URL:       fmt.Sprintf("https://www.amazon.com/dp/%s", productID),
				Zipcode:   "10001",
				Priority:  1,
			}

			result, err := suite.crawlerClient.SubmitCrawlTask(ctx, request)
			if err != nil {
				errors <- err
				return
			}
			results <- result
		}(i+1, productID)
	}

	// 收集结果
	successCount := 0
	errorCount := 0
	timeout := time.After(suite.config.Timeout)

	for i := 0; i < len(testProducts); i++ {
		select {
		case result := <-results:
			if result.Success {
				successCount++
				suite.logger.Infof("✅ 并发任务成功: TaskID=%d, 产品=%s",
					result.TaskID, result.Product.Title)
			} else {
				errorCount++
				suite.logger.Warnf("⚠️ 并发任务失败: TaskID=%d, 错误=%s",
					result.TaskID, result.Error)
			}
		case err := <-errors:
			errorCount++
			suite.logger.Errorf("❌ 并发任务错误: %v", err)
		case <-timeout:
			t.Skip("⏰ 并发测试超时，可能爬虫节点未运行")
			return
		}
	}

	suite.logger.Infof("📊 并发测试结果: 成功=%d, 失败=%d, 总计=%d",
		successCount, errorCount, len(testProducts))

	// 至少应该有一些结果（成功或失败都算）
	assert.Equal(t, len(testProducts), successCount+errorCount, "应该收到所有任务的结果")
}

// testErrorHandling 测试错误处理
func (suite *IntegrationTestSuite) testErrorHandling(t *testing.T) {
	suite.logger.Info("🧪 开始测试错误处理")

	// 使用无效的产品ID测试错误处理
	request := &crawler.CrawlRequest{
		TaskID:    99999,
		TenantID:  1,
		StoreID:   617,
		Platform:  "amazon",
		Region:    "us",
		ProductID: "INVALID_PRODUCT_ID",
		URL:       "https://www.amazon.com/dp/INVALID_PRODUCT_ID",
		Zipcode:   "10001",
		Priority:  1,
	}

	ctx, cancel := context.WithTimeout(context.Background(), suite.config.Timeout)
	defer cancel()

	result, err := suite.crawlerClient.SubmitCrawlTask(ctx, request)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			t.Skip("⏰ 爬虫节点未运行，跳过错误处理测试")
			return
		}
		// 网络错误等系统级错误
		suite.logger.Infof("✅ 系统级错误处理正常: %v", err)
		return
	}

	// 应该收到结果，但可能是失败的
	assert.NotNil(t, result, "应该收到结果")
	assert.Equal(t, request.TaskID, result.TaskID, "任务ID应该匹配")

	if !result.Success {
		assert.NotEmpty(t, result.Error, "失败时应该有错误信息")
		suite.logger.Infof("✅ 业务级错误处理正常: %s", result.Error)
	} else {
		suite.logger.Info("ℹ️ 意外成功，可能产品ID实际存在")
	}
}

// testTimeoutHandling 测试超时处理
func (suite *IntegrationTestSuite) testTimeoutHandling(t *testing.T) {
	suite.logger.Info("🧪 开始测试超时处理")

	// 设置很短的超时时间
	suite.crawlerClient.SetTimeout(1 * time.Second)
	defer suite.crawlerClient.SetTimeout(30 * time.Second) // 恢复默认超时

	request := &crawler.CrawlRequest{
		TaskID:    88888,
		TenantID:  1,
		StoreID:   617,
		Platform:  "amazon",
		Region:    "us",
		ProductID: "B08BKKQ2NL",
		URL:       "https://www.amazon.com/dp/B08BKKQ2NL",
		Zipcode:   "10001",
		Priority:  1,
	}

	ctx := context.Background()
	start := time.Now()
	result, err := suite.crawlerClient.SubmitCrawlTask(ctx, request)
	duration := time.Since(start)

	// 应该在大约1秒内超时
	assert.True(t, duration < 2*time.Second, "应该快速超时")

	if err != nil {
		assert.Contains(t, err.Error(), "超时", "错误信息应该包含超时")
		suite.logger.Infof("✅ 超时处理正常: %v, 耗时=%v", err, duration)
	} else if result != nil {
		// 如果在超时前完成了，也是正常的
		suite.logger.Infof("ℹ️ 任务在超时前完成: TaskID=%d, 耗时=%v", result.TaskID, duration)
	}
}

// TestRabbitMQConnection 测试RabbitMQ连接
func TestRabbitMQConnection(t *testing.T) {
	logger := logrus.New()
	rabbitmqURL := getEnvOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")

	logger.Info("🧪 测试RabbitMQ连接")

	// 创建连接配置
	connConfig := rabbitmq.ConnectionConfig{
		URL:               rabbitmqURL,
		ReconnectInterval: 5 * time.Second,
		MaxReconnectTries: 3,
	}

	// 创建连接管理器
	connManager := rabbitmq.NewConnectionManager(connConfig, logger)

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := connManager.Connect(ctx)
	if err != nil {
		t.Skipf("⏰ 无法连接到RabbitMQ: %v", err)
		return
	}
	defer connManager.Close()

	assert.True(t, connManager.IsConnected(), "应该已连接到RabbitMQ")
	logger.Info("✅ RabbitMQ连接测试通过")
}

// TestServiceManagerLifecycle 测试服务管理器生命周期
func TestServiceManagerLifecycle(t *testing.T) {
	logger := logrus.New()

	logger.Info("🧪 测试服务管理器生命周期")

	// 加载配置文件
	cfg := config.LoadConfigFromFile("config/config-dev.yaml")
	if cfg == nil || cfg.RabbitMQ == nil {
		t.Skip("⏰ 无法加载配置文件或RabbitMQ配置为空")
		return
	}

	// 创建服务管理器
	serviceManager, err := messaging.NewServiceManager(cfg.RabbitMQ, logger)
	if err != nil {
		t.Skipf("⏰ 无法创建服务管理器: %v", err)
		return
	}

	// 测试启动
	ctx := context.Background()
	err = serviceManager.Start(ctx)
	if err != nil {
		t.Skipf("⏰ 无法启动服务管理器: %v", err)
		return
	}

	assert.True(t, serviceManager.IsStarted(), "服务管理器应该已启动")
	logger.Info("✅ 服务管理器启动成功")

	// 测试停止
	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = serviceManager.Stop(stopCtx)
	assert.NoError(t, err, "停止服务管理器不应该出错")
	assert.False(t, serviceManager.IsStarted(), "服务管理器应该已停止")
	logger.Info("✅ 服务管理器停止成功")
}

// BenchmarkCrawlerClient 性能基准测试
func BenchmarkCrawlerClient(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // 减少日志输出

	rabbitmqURL := getEnvOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")

	crawlerClient, err := crawler.NewDistributedCrawlerClient(rabbitmqURL, logger)
	if err != nil {
		b.Skipf("无法创建爬虫客户端: %v", err)
		return
	}
	defer crawlerClient.Close()

	// 设置较短的超时以加快测试
	crawlerClient.SetTimeout(5 * time.Second)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		taskID := int64(1)
		for pb.Next() {
			request := &crawler.CrawlRequest{
				TaskID:    taskID,
				TenantID:  1,
				StoreID:   617,
				Platform:  "amazon",
				Region:    "us",
				ProductID: "B08BKKQ2NL",
				URL:       "https://www.amazon.com/dp/B08BKKQ2NL",
				Zipcode:   "10001",
				Priority:  1,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, _ = crawlerClient.SubmitCrawlTask(ctx, request)
			cancel()
			taskID++
		}
	})
}

// getEnvOrDefault 获取环境变量或默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
