// Package main 提供RabbitMQ消费者主程序
package main

import (
	"context"
	"flag"
	"strings"

	"task-processor/internal/app/consumer"
	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	"task-processor/internal/pkg/appenv"
)

var (
	appConfig = flag.String("app-config", "config/config-dev.yaml", "应用配置文件路径")
	logLevel  = flag.String("log-level", "info", "日志级别")
)

// 版本信息（通过 -ldflags 在编译时注入）
var (
	appVersion = "1.0.0" // 默认版本，编译时会被覆盖
	buildTime  = "unknown"
)

func main() {
	flag.Parse()

	// 设置日志（使用统一的日志设置函数）
	logger := appenv.SetupLoggerWithLevel(*logLevel)

	// 打印版本信息（统一格式）
	appenv.PrintVersionInfo(logger, appenv.VersionInfo{
		Version:   appVersion,
		BuildTime: buildTime,
	})

	logger.Info("🚀 启动RabbitMQ消费者...")
	logger.Infof("📋 配置文件路径: %s", *appConfig)

	// 修复：如果配置文件路径不包含扩展名，自动添加 .yaml
	configPath := *appConfig
	if !strings.HasSuffix(configPath, ".yaml") && !strings.HasSuffix(configPath, ".yml") {
		configPath = configPath + ".yaml"
		logger.Infof("⚠️  配置路径缺少扩展名，自动补全为: %s", configPath)
	}

	// 加载应用配置（使用统一的配置加载函数）
	appCfg := config.LoadConfigWithFallback(configPath, logger)

	// 应用配置文件中的日志设置（级别、格式、文件输出）
	if err := appenv.ApplyLoggingConfig(logger, appenv.LoggingConfig{
		Level:        appCfg.Logging.Level,
		Format:       appCfg.Logging.Format,
		File:         appCfg.Logging.File,
		SplitByLevel: appCfg.Logging.SplitByLevel,
	}); err != nil {
		logger.Warnf("⚠️  应用日志配置失败: %v", err)
	}

	// 验证RabbitMQ配置
	if appCfg.RabbitMQ == nil {
		logger.Fatal("❌ RabbitMQ配置未启用，请在配置文件中设置 rabbitmq.enabled: true")
	}

	// 创建服务管理器
	serviceManager, err := consumer.NewServiceManager(appCfg.RabbitMQ, logger)
	if err != nil {
		logger.Fatalf("❌ 创建服务管理器失败: %v", err)
	}

	// 创建平台注册器并注册所有处理器（不指定平台，根据任务动态判断）
	platformRegistry := consumer.NewPlatformRegistry(appCfg, logger, "")
	ctx := context.Background()
	if err := platformRegistry.RegisterAllProcessors(ctx, serviceManager); err != nil {
		logger.Fatalf("❌ 注册平台处理器失败: %v", err)
	}

	// 注册爬虫处理器（集成分布式爬虫服务，复用共享的Amazon处理器）
	logger.Info("🕷️  集成分布式爬虫服务...")
	crawlerRegistry := consumer.NewCrawlerRegistry(appCfg, logger, serviceManager.GetClient())
	if err := crawlerRegistry.RegisterCrawlerProcessor(serviceManager, platformRegistry.GetSharedAmazonProcessor()); err != nil {
		logger.Fatalf("❌ 注册爬虫处理器失败: %v", err)
	}

	// 注入店铺亲和性组件（storeAPI + 本节点负责的店铺列表）
	ownedStores := appCfg.RabbitMQ.Node.OwnedStores
	if len(ownedStores) > 0 {
		logger.Infof("🏪 本节点负责的店铺: %v", ownedStores)
		serviceManager.SetStoreComponents(nil, ownedStores, nil)
	} else {
		logger.Warn("⚠️  未配置 rabbitmq.node.ownedStores，本节点将订阅平台级队列（处理所有店铺任务）")
	}

	// 注入调度服务（核价、同步等定时任务），根据配置决定是否启用
	if appCfg.Platforms.Temu.SchedulerEnabled || appCfg.Platforms.Shein.SchedulerEnabled {
		schedulerSvc := runner.NewSchedulerServiceWithAmazon(
			logger,
			platformRegistry.GetManagementClient(),
			appCfg,
			platformRegistry.GetSharedAmazonProcessor(),
		)
		serviceManager.SetSchedulerService(schedulerSvc)
		logger.Info("✅ 调度服务已注入（自动核价/同步等任务）")
	}

	// 启动服务
	if err := serviceManager.Start(ctx); err != nil {
		logger.Fatalf("❌ 启动服务失败: %v", err)
	}

	logger.Info("✅ RabbitMQ消费者启动完成（集成上架服务 + 爬虫服务）")
	logger.Info("📊 服务信息:")
	logger.Info("   - 上架服务: Amazon/TEMU/SHEIN 平台上架")
	logger.Info("   - 爬虫服务: Amazon/1688 分布式爬虫")
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
