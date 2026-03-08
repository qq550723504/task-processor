// Package main 提供RabbitMQ消费者主程序
package main

import (
	"context"
	"flag"
	"fmt"

	"task-processor/internal/app/messaging"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/pkg/management"
	"task-processor/internal/pkg/utils"
	platformAmazon "task-processor/internal/platforms/amazon"
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

// 版本信息（通过 -ldflags 在编译时注入）
var (
	appVersion = "1.0.0" // 默认版本，编译时会被覆盖
	buildTime  = "unknown"
)

func main() {
	flag.Parse()

	// 设置日志（使用统一的日志设置函数）
	logger := utils.SetupLoggerWithLevel(*logLevel)

	// 打印版本信息（统一格式）
	utils.PrintVersionInfo(logger, utils.VersionInfo{
		Version:   appVersion,
		BuildTime: buildTime,
	})

	logger.Info("🚀 启动RabbitMQ消费者...")

	// 加载应用配置（使用统一的配置加载函数）
	appCfg := config.LoadConfigWithFallback(*appConfig, logger)

	// 创建服务管理器
	serviceManager, err := messaging.NewServiceManager(*configPath, logger)
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

// registerProcessors 注册任务处理器
func registerProcessors(serviceManager *messaging.ServiceManager, appCfg *config.Config, platformsStr string, logger *logrus.Logger) error {
	ctx := context.Background()

	// 解析平台列表（使用统一的工具函数）
	enabledPlatforms := utils.ParsePlatformList(platformsStr)
	logger.Infof("🔧 启用的平台: %v", enabledPlatforms)

	// 创建管理客户端
	managementClient := management.NewClientManager(&appCfg.Management)

	// 创建共享的Amazon处理器（所有平台共用）
	var sharedAmazonProcessor *amazon.AmazonProcessor

	sharedAmazonProcessor = createSharedAmazonProcessor(appCfg, logger)

	// 注册Amazon平台处理器（上架）
	if utils.ContainsPlatform(enabledPlatforms, "amazon") {
		logger.Info("📦 注册Amazon平台处理器...")

		// 使用完整的Amazon平台处理器（用于上架）
		amazonProcessor := platformAmazon.NewProcessor(ctx, appCfg, logger)

		if err := serviceManager.RegisterProcessor("amazon", amazonProcessor); err != nil {
			return fmt.Errorf("注册Amazon平台处理器失败: %w", err)
		}
		logger.Info("✅ Amazon平台处理器注册成功")
	}

	// 注册TEMU处理器
	if utils.ContainsPlatform(enabledPlatforms, "temu") {
		logger.Info("📦 注册TEMU处理器...")
		temuProcessor, err := temu.NewTemuProcessor(ctx, appCfg, logger, managementClient, sharedAmazonProcessor)
		if err != nil {
			return fmt.Errorf("创建TEMU处理器失败: %w", err)
		}
		if err := serviceManager.RegisterProcessor("temu", temuProcessor); err != nil {
			return fmt.Errorf("注册TEMU处理器失败: %w", err)
		}
		logger.Info("✅ TEMU处理器注册成功")
	}

	// 注册SHEIN处理器
	if utils.ContainsPlatform(enabledPlatforms, "shein") {
		logger.Info("📦 注册SHEIN处理器...")
		sheinProcessor, err := pipeline.NewSheinProcessor(ctx, appCfg, logger, managementClient, sharedAmazonProcessor)
		if err != nil {
			return fmt.Errorf("创建SHEIN处理器失败: %w", err)
		}
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
		cfg.Browser.PoolSize = 3 // 至少需要1个实例
	}

	// 创建Amazon爬虫处理器
	amazonProcessor := amazon.NewAmazonProcessor(cfg)

	logger.Info("✅ 共享Amazon爬虫处理器创建成功")
	return amazonProcessor
}
