// Package main 提供RabbitMQ消费者主程序
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/pkg/management"
	amazonPlatform "task-processor/internal/platforms/amazon"
	"task-processor/internal/platforms/shein/service/pipeline"
	"task-processor/internal/platforms/temu"

	"github.com/sirupsen/logrus"
)

var (
	configPath = flag.String("config", "config/rabbitmq-config.yaml", "RabbitMQ配置文件路径")
	appConfig  = flag.String("app-config", "config/config-dev.yaml", "应用配置文件路径")
	logLevel   = flag.String("log-level", "info", "日志级别")
	platforms  = flag.String("platforms", "amazon,temu,shein", "启用的平台，用逗号分隔")
)

func main() {
	flag.Parse()

	// 设置日志
	logger := setupLogger(*logLevel)
	logger.Info("🚀 启动RabbitMQ消费者...")

	// 加载应用配置
	appCfg, err := loadAppConfig(*appConfig, logger)
	if err != nil {
		logger.Fatalf("❌ 加载应用配置失败: %v", err)
	}

	// 创建服务管理器
	serviceManager, err := rabbitmq.NewServiceManager(*configPath, logger)
	if err != nil {
		logger.Fatalf("❌ 创建服务管理器失败: %v", err)
	}

	// 注册任务处理器
	if err := registerProcessors(serviceManager, appCfg, *platforms, logger); err != nil {
		logger.Fatalf("❌ 注册任务处理器失败: %v", err)
	}

	// 启动服务
	ctx := context.Background()
	if err := serviceManager.Start(ctx); err != nil {
		logger.Fatalf("❌ 启动服务失败: %v", err)
	}

	logger.Info("✅ RabbitMQ消费者启动完成")
	logger.Info("📊 监控地址:")
	logger.Info("   - 健康检查: http://localhost:8081/health")
	logger.Info("   - 就绪检查: http://localhost:8081/ready")
	logger.Info("   - 指标监控: http://localhost:8082/metrics")
	logger.Info("   - 统计信息: http://localhost:8082/stats")
	logger.Info("🔄 按 Ctrl+C 优雅关闭服务")

	// 等待服务管理器完成（它内部会处理信号）
	serviceManager.Wait()
	logger.Info("程序已退出")
}

// setupLogger 设置日志器
func setupLogger(level string) *logrus.Logger {
	logger := logrus.New()

	// 设置日志级别
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	logger.SetLevel(logLevel)

	// 设置日志格式
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})

	return logger
}

// loadAppConfig 加载应用配置
func loadAppConfig(configPath string, logger *logrus.Logger) (*config.Config, error) {
	// 检查配置文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		logger.Warnf("⚠️  配置文件不存在: %s，使用默认配置", configPath)
		return createDefaultConfig(), nil
	}

	// 使用实际的配置加载逻辑
	logger.Infof("📄 加载应用配置: %s", configPath)
	return config.LoadConfigFromFile(configPath), nil
}

// createDefaultConfig 创建默认配置
func createDefaultConfig() *config.Config {
	return &config.Config{
		Worker: config.WorkerConfig{
			TaskInterval: 30,
		},
		Management: config.ManagementConfig{
			BaseURL: "http://localhost:8080",
		},
		Amazon: config.AmazonConfig{
			DataFreshnessDays: 7,
		},
	}
}

// registerProcessors 注册任务处理器
func registerProcessors(serviceManager *rabbitmq.ServiceManager, appCfg *config.Config, platformsStr string, logger *logrus.Logger) error {
	ctx := context.Background()

	// 解析平台列表
	enabledPlatforms := strings.Split(platformsStr, ",")
	for i, platform := range enabledPlatforms {
		enabledPlatforms[i] = strings.TrimSpace(platform)
	}

	logger.Infof("🔧 启用的平台: %v", enabledPlatforms)

	// 创建管理客户端
	managementClient := management.NewClientManager(&appCfg.Management)

	// 注册Amazon处理器
	if containsPlatform(enabledPlatforms, "amazon") {
		logger.Info("📦 注册Amazon处理器...")
		amazonProcessor := amazonPlatform.NewProcessor(ctx, appCfg, logger)
		if err := serviceManager.RegisterProcessor("amazon", amazonProcessor); err != nil {
			return fmt.Errorf("注册Amazon处理器失败: %w", err)
		}
		logger.Info("✅ Amazon处理器注册成功")
	}

	// 注册TEMU处理器
	if containsPlatform(enabledPlatforms, "temu") {
		logger.Info("📦 注册TEMU处理器...")

		// 创建共享的Amazon处理器（TEMU需要）
		sharedAmazonProcessor := createSharedAmazonProcessor(appCfg, logger)

		temuProcessor := temu.NewTemuProcessor(ctx, appCfg, logger, managementClient, sharedAmazonProcessor)
		if err := serviceManager.RegisterProcessor("temu", temuProcessor); err != nil {
			return fmt.Errorf("注册TEMU处理器失败: %w", err)
		}
		logger.Info("✅ TEMU处理器注册成功")
	}

	// 注册SHEIN处理器
	if containsPlatform(enabledPlatforms, "shein") {
		logger.Info("📦 注册SHEIN处理器...")

		// 创建共享的Amazon处理器（SHEIN需要）
		sharedAmazonProcessor := createSharedAmazonProcessor(appCfg, logger)

		sheinProcessor := pipeline.NewSheinProcessor(ctx, appCfg, logger, managementClient, sharedAmazonProcessor)
		if err := serviceManager.RegisterProcessor("shein", sheinProcessor); err != nil {
			return fmt.Errorf("注册SHEIN处理器失败: %w", err)
		}
		logger.Info("✅ SHEIN处理器注册成功")
	}

	return nil
}

// createSharedAmazonProcessor 创建共享的Amazon处理器
func createSharedAmazonProcessor(cfg *config.Config, logger *logrus.Logger) *amazon.AmazonProcessor {
	logger.Info("🔧 创建共享Amazon爬虫处理器...")

	// 确保浏览器配置合理
	if cfg.Browser.PoolSize <= 0 {
		cfg.Browser.PoolSize = 1 // 至少需要1个实例
	}

	// 创建Amazon爬虫处理器
	amazonProcessor := amazon.NewAmazonProcessor(cfg)

	logger.Info("✅ 共享Amazon爬虫处理器创建成功")
	return amazonProcessor
}

// containsPlatform 检查平台列表是否包含指定平台
func containsPlatform(platforms []string, platform string) bool {
	for _, p := range platforms {
		if strings.EqualFold(p, platform) {
			return true
		}
	}
	return false
}
