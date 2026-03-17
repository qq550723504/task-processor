// Package main 提供爬虫RabbitMQ消费者主程序
package main

import (
	"context"
	"flag"

	"task-processor/internal/app/consumer"
	"task-processor/internal/core/config"
	"task-processor/internal/pkg/appenv"
)

var (
	appConfig = flag.String("app-config", "config/config-dev.yaml", "应用配置文件路径")
	logLevel  = flag.String("log-level", "info", "日志级别")
)

// 版本信息
var (
	appVersion = "1.0.0"
	buildTime  = "unknown"
)

func main() {
	flag.Parse()

	// 设置日志
	logger := appenv.SetupLoggerWithLevel(*logLevel)

	// 打印版本信息
	appenv.PrintVersionInfo(logger, appenv.VersionInfo{
		Version:   appVersion,
		BuildTime: buildTime,
	})

	logger.Info("🚀 启动Amazon爬虫消费者...")

	// 加载应用配置
	appCfg := config.LoadConfigWithFallback(*appConfig, logger)

	// 验证RabbitMQ配置
	if appCfg.RabbitMQ == nil {
		logger.Fatal("❌ RabbitMQ配置未启用，请在配置文件中设置 rabbitmq.enabled: true")
	}

	// 创建服务管理器
	serviceManager, err := consumer.NewServiceManager(appCfg.RabbitMQ, logger)
	if err != nil {
		logger.Fatalf("❌ 创建服务管理器失败: %v", err)
	}

	// 创建爬虫注册器并注册处理器
	crawlerRegistry := consumer.NewCrawlerRegistry(appCfg, logger, serviceManager.GetClient())
	if err := crawlerRegistry.RegisterCrawlerProcessor(serviceManager, nil); err != nil {
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
